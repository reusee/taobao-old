package taobao

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/gob"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/reusee/dsfile"
	"github.com/ugorji/go/codec"
)

var _ Backend = new(FileBackend)

var codecHandle = new(codec.CborHandle)

type FileBackend struct {
	date    string
	dataDir string

	fgCats          IntSet
	fgCatsFile      *dsfile.File
	fgCatsFileDirty uint32

	bgCats          map[int]*BgCat
	bgCatsFile      *dsfile.File
	bgCatsFileDirty uint32

	collected map[Job]bool

	itemsFile         *os.File
	itemsLock         *sync.Mutex
	itemsFileScanOnce *sync.Once

	closed chan struct{}
}

type EntryHeader struct {
	Cat  uint64
	Page uint8
	Len  uint32
}

func NewFileBackend(now time.Time) (b *FileBackend, err error) {
	defer ct(&err)
	date := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())
	dataDir := "data"

	b = &FileBackend{
		date:              date,
		dataDir:           dataDir,
		fgCats:            NewIntSet(),
		bgCats:            make(map[int]*BgCat),
		closed:            make(chan struct{}),
		collected:         make(map[Job]bool),
		itemsFileScanOnce: new(sync.Once),
	}

	b.fgCatsFile, err = dsfile.New(&b.fgCats, filepath.Join(dataDir, "fgcats"),
		new(dsfile.Cbor), dsfile.NewFileLocker(filepath.Join(dataDir, "fgcats.lock")))
	ce(err, "fgcats file")

	b.bgCatsFile, err = dsfile.New(&b.bgCats, filepath.Join(dataDir, "bgcats"),
		new(dsfile.Cbor), dsfile.NewFileLocker(filepath.Join(dataDir, "bgcats.lock")))
	ce(err, "bgcats file")

	b.itemsLock = new(sync.Mutex)
	itemsFile, err := os.OpenFile(filepath.Join(dataDir, sp("%s-items", date)),
		os.O_RDWR|os.O_CREATE, 0644)
	ce(err, "open items file")
	b.itemsFile = itemsFile

	go func() {
		t := time.NewTicker(time.Second*3 + time.Millisecond*500)
		for {
			select {
			case <-t.C:
				withLock(b.itemsLock, func() {
					err := itemsFile.Sync()
					ce(err, "sync items file")
				})
			case <-b.closed:
				return
			}
		}
	}()

	return b, nil
}

func (b *FileBackend) scanItemsFile() (err error) {
	defer ct(&err)
	pt("scanning items file\n")
	t0 := time.Now()
	b.itemsFile.Seek(0, os.SEEK_SET)
	doneJobs := 0
	for {
		var header EntryHeader
		err = binary.Read(b.itemsFile, binary.LittleEndian, &header)
		if err == io.EOF {
			err = nil
			break
		}
		ce(err, "read entry len")
		_, err = b.itemsFile.Seek(int64(header.Len), os.SEEK_CUR)
		ce(err, "seek entry")
		job := Job{
			Cat:  int(header.Cat),
			Page: int(header.Page),
		}
		b.collected[job] = true
		doneJobs++
	}
	pt("finish scanning in %v, %d jobs done\n", time.Now().Sub(t0), doneJobs)
	return
}

func (b *FileBackend) Close() {
	b.fgCatsFile.Save()
	b.fgCatsFile.Close()
	b.bgCatsFile.Save()
	b.bgCatsFile.Close()
	b.itemsFile.Close()
}

func (b *FileBackend) AddBgCat(cat Cat) (err error) {
	defer ct(&err)
	if _, ok := b.bgCats[cat.Cat]; ok {
		return nil
	}
	b.bgCats[cat.Cat] = &BgCat{
		Cat:  cat.Cat,
		Name: cat.Name,
		Subs: NewIntSet(),
	}
	b.bgCats[cat.Parent].Subs.Add(cat.Cat)
	if atomic.AddUint32(&b.bgCatsFileDirty, 1)%128 == 0 {
		err = b.bgCatsFile.Save()
		ce(err, "save bgcats file")
	}
	return
}

func (b *FileBackend) GetBgCatLastUpdated(cat int) (t time.Time, err error) {
	if cat, ok := b.bgCats[cat]; ok {
		t = cat.LastUpdated
	}
	return
}

func (b *FileBackend) SetBgCatLastUpdated(cat int, t time.Time) error {
	b.bgCats[cat].LastUpdated = t
	if atomic.AddUint32(&b.bgCatsFileDirty, 1)%128 == 0 {
		return b.bgCatsFile.Save()
	}
	return nil
}

func (b *FileBackend) AddFgCat(cat Cat) (err error) {
	defer ct(&err)
	b.fgCats.Add(cat.Cat)
	if atomic.AddUint32(&b.fgCatsFileDirty, 1)%128 == 0 {
		err = b.fgCatsFile.Save()
		ce(err, "save fgcats file")
	}
	return
}

func (b *FileBackend) GetFgCats() (cats []Cat, err error) {
	for cat := range b.fgCats {
		cats = append(cats, Cat{
			Cat: cat,
		})
	}
	return
}

func (b *FileBackend) AddItems(items []Item, job Job) (err error) {
	defer ct(&err)
	b.itemsFileScanOnce.Do(func() {
		err := b.scanItemsFile()
		ce(err, "scan items file")
	})
	buf := new(bytes.Buffer)
	w := gzip.NewWriter(buf)
	err = codec.NewEncoder(w, codecHandle).Encode(items)
	ce(err, "gob encode")
	err = w.Close()
	ce(err, "close write")
	bs := buf.Bytes()
	withLock(b.itemsLock, func() {
		header := EntryHeader{
			Cat:  uint64(job.Cat),
			Page: uint8(job.Page),
			Len:  uint32(len(bs)),
		}
		err = binary.Write(b.itemsFile, binary.LittleEndian, header)
		ce(err, "write items file entry header")
		_, err = b.itemsFile.Write(bs)
		ce(err, "write items entry")
	})
	return nil
}

func (b *FileBackend) IsCollected(job Job) bool {
	b.itemsFileScanOnce.Do(func() {
		err := b.scanItemsFile()
		ce(err, "scan items file")
	})
	return b.collected[job]
}

func (b *FileBackend) iterItems(fn func(Item) bool) {
	itemsFile, err := os.Open(filepath.Join(b.dataDir, sp("%s-items", b.date)))
	ce(err, "open items file")
	kill := make(chan struct{})

	// read entries
	bss := make(chan []byte)
	go func() {
	loop:
		for {
			var header EntryHeader
			err := binary.Read(itemsFile, binary.LittleEndian, &header)
			if err == io.EOF {
				err = nil
				break
			}
			ce(err, "read header")
			bs := make([]byte, header.Len)
			_, err = io.ReadFull(itemsFile, bs)
			ce(err, "read data")
			select {
			case bss <- bs:
			case <-kill:
				break loop
			}
		}
		close(bss)
	}()

	// decode
	itemsChan := make(chan []Item)
	wg := new(sync.WaitGroup)
	ncpu := runtime.NumCPU()
	wg.Add(ncpu)
	decode := func() {
		defer wg.Done()
		for bs := range bss {
			r, err := gzip.NewReader(bytes.NewReader(bs))
			ce(err, "new gzip reader")
			var items []Item
			err = codec.NewDecoder(r, codecHandle).Decode(&items)
			ce(err, "decode")
			select {
			case itemsChan <- items:
			case <-kill:
				return
			}
		}
	}
	for i := 0; i < ncpu; i++ {
		go decode()
	}
	go func() {
		wg.Wait()
		close(itemsChan)
	}()

	// process items
	var itemIdSet [1000]IntSet
	for i := 0; i < 1000; i++ {
		itemIdSet[i] = NewIntSet()
	}
loop:
	for items := range itemsChan {
		for _, item := range items {
			// dedup
			slot := item.Nid % 1000
			if itemIdSet[slot].Has(item.Nid) {
				continue
			}
			itemIdSet[slot].Add(item.Nid)
			// callback
			if !fn(item) {
				close(kill)
				break loop
			}
		}
	}
}

func (b *FileBackend) PostProcess() {
	t0 := time.Now()
	defer func() {
		pt("post processed in %v\n", time.Now().Sub(t0))
	}()

	// cat stats
	catStats := make(map[int]*CatStat)
	b.iterItems(func(item Item) bool {
		if _, ok := catStats[item.Category]; !ok {
			catStats[item.Category] = new(CatStat)
		}
		catStats[item.Category].Items += 1
		catStats[item.Category].Sales += item.Sales
		return true
	})
	catStatsFile, err := os.Create(filepath.Join(b.dataDir, sp("%s-cat-stats", b.date)))
	ce(err, "create cat stats file")
	defer catStatsFile.Close()
	err = gob.NewEncoder(catStatsFile).Encode(catStats)
	ce(err, "save cat stats")
}

func (b *FileBackend) Stats() {
	// read cat stats
	catStats := make(map[int]*CatStat)
	catStatsFile, err := os.Open(filepath.Join(b.dataDir, sp("%s-cat-stats", b.date)))
	ce(err, "open cat stats file")
	defer catStatsFile.Close()
	err = gob.NewDecoder(catStatsFile).Decode(&catStats)
	ce(err, "decode cat stats")

	// collect unknown bgcats
	unknownCats := Ints([]int{})
	if false {
		knownCats := Ints([]int{})
		for cat, _ := range catStats {
			if _, ok := b.bgCats[cat]; !ok {
				unknownCats = append(unknownCats, cat)
			} else {
				knownCats = append(knownCats, cat)
			}
		}
		unknownCats.Sort(func(a, b int) bool {
			return catStats[a].Sales > catStats[b].Sales
		})
		knownCats.Sort(func(a, b int) bool {
			return catStats[a].Sales > catStats[b].Sales
		})
	}

	// show items in unknown cats
	if false {
		for _, cat := range unknownCats {
			n := 0
			b.iterItems(func(item Item) bool {
				if item.Category != cat {
					return true
				}
				pt("%s\n", item.Title)
				n++
				return n < 30
			})
			pt("\n")
		}
	}

	n := 0
	b.iterItems(func(item Item) bool {
		n++
		return true
	})
	pt("%d\n", n)

}

func (b *FileBackend) LogClient(info ClientInfo, state ClientState) {
}
