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
		ce(err, "read entry header")
		_, err = b.itemsFile.Seek(int64(header.NidsLen+header.Len1+header.Len2+header.Len3), os.SEEK_CUR)
		ce(err, "seek")
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

func encode(o interface{}) (bs []byte, err error) {
	defer ct(&err)
	buf := new(bytes.Buffer)
	w := gzip.NewWriter(buf)
	err = codec.NewEncoder(w, codecHandle).Encode(o)
	ce(err, "encode")
	err = w.Close()
	ce(err, "close write")
	return buf.Bytes(), nil
}

func (b *FileBackend) AddItems(items []Item, job Job) (err error) {
	defer ct(&err)
	b.itemsFileScanOnce.Do(func() {
		err := b.scanItemsFile()
		ce(err, "scan items file")
	})
	var nids []int
	var c1 []Item1
	var c2 []Item2
	var c3 []Item3
	for _, item := range items {
		nids = append(nids, item.Nid)
		c1 = append(c1, item.Item1)
		c2 = append(c2, item.Item2)
		c3 = append(c3, item.Item3)
	}
	nidsBs, err := encode(nids)
	ce(err, "encode nids")
	c1Bs, err := encode(c1)
	ce(err, "encode col set 1")
	c2Bs, err := encode(c2)
	ce(err, "encode col set 2")
	c3Bs, err := encode(c3)
	ce(err, "encode col set 3")
	withLock(b.itemsLock, func() {
		header := EntryHeader{
			Cat:     uint64(job.Cat),
			Page:    uint8(job.Page),
			NidsLen: uint32(len(nidsBs)),
			Len1:    uint32(len(c1Bs)),
			Len2:    uint32(len(c2Bs)),
			Len3:    uint32(len(c3Bs)),
		}
		err = binary.Write(b.itemsFile, binary.LittleEndian, header)
		ce(err, "write items file entry header")
		_, err = b.itemsFile.Write(nidsBs)
		ce(err, "write nids")
		_, err = b.itemsFile.Write(c1Bs)
		ce(err, "write col set 1")
		_, err = b.itemsFile.Write(c2Bs)
		ce(err, "write col set 2")
		_, err = b.itemsFile.Write(c3Bs)
		ce(err, "write col set 3")
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

const (
	ColSet1 uint8 = 1
	ColSet2 uint8 = 2
	ColSet3 uint8 = 4
)

func decode(bs []byte, target interface{}) (err error) {
	defer ct(&err)
	r, err := gzip.NewReader(bytes.NewReader(bs))
	ce(err, "new gzip reader")
	err = codec.NewDecoder(r, codecHandle).Decode(target)
	ce(err, "decode")
	return
}

func (b *FileBackend) iterItems(colSets uint8, fn func(nid int, c1 *Item1, c2 *Item2, c3 *Item3) bool) {
	itemsFile, err := os.Open(filepath.Join(b.dataDir, sp("%s-items", b.date)))
	ce(err, "open items file")
	kill := make(chan struct{})

	// read entries
	rawss := make(chan [4][]byte)
	go func() {
	loop:
		for {
			// decode header
			var header EntryHeader
			err := binary.Read(itemsFile, binary.LittleEndian, &header)
			if err == io.EOF {
				err = nil
				break
			}
			ce(err, "read header")
			// decode column sets
			raw := [4][]byte{}
			bs := make([]byte, header.NidsLen)
			_, err = io.ReadFull(itemsFile, bs)
			ce(err, "read nids bytes")
			raw[0] = bs
			if colSets&ColSet1 != 0 {
				bs = make([]byte, header.Len1)
				_, err = io.ReadFull(itemsFile, bs)
				ce(err, "read col set 1")
				raw[1] = bs
			} else {
				_, err = itemsFile.Seek(int64(header.Len1), os.SEEK_CUR)
				ce(err, "seek")
			}
			if colSets&ColSet2 != 0 {
				bs = make([]byte, header.Len2)
				_, err = io.ReadFull(itemsFile, bs)
				ce(err, "read col set 2")
				raw[2] = bs
			} else {
				_, err = itemsFile.Seek(int64(header.Len2), os.SEEK_CUR)
				ce(err, "seek")
			}
			if colSets&ColSet3 != 0 {
				bs = make([]byte, header.Len3)
				_, err = io.ReadFull(itemsFile, bs)
				ce(err, "read col set 3")
				raw[3] = bs
			} else {
				_, err = itemsFile.Seek(int64(header.Len3), os.SEEK_CUR)
				ce(err, "seek")
			}
			// send
			select {
			case rawss <- raw:
			case <-kill:
				break loop
			}
		}
		close(rawss)
	}()

	// decode
	type Info struct {
		Nids []int
		C1   []Item1
		C2   []Item2
		C3   []Item3
	}
	itemsChan := make(chan Info)
	wg := new(sync.WaitGroup)
	ncpu := runtime.NumCPU()
	wg.Add(ncpu)
	decode := func() {
		defer wg.Done()
		for raw := range rawss {
			var info Info
			err = decode(raw[0], &info.Nids)
			ce(err, "decode nids")
			if colSets&ColSet1 != 0 {
				err = decode(raw[1], &info.C1)
				ce(err, "decode c1")
			}
			if colSets&ColSet2 != 0 {
				err = decode(raw[2], &info.C2)
				pt("%d\n", len(raw[2]))
				ce(err, "decode c2")
			}
			if colSets&ColSet3 != 0 {
				err = decode(raw[3], &info.C3)
				ce(err, "decode c3")
			}
			select {
			case itemsChan <- info:
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
	for info := range itemsChan {
		for n, nid := range info.Nids {
			// dedup
			slot := nid % 1000
			if itemIdSet[slot].Has(nid) {
				continue
			}
			itemIdSet[slot].Add(nid)
			// callback
			var c1 *Item1
			var c2 *Item2
			var c3 *Item3
			if colSets&ColSet1 != 0 {
				c1 = &info.C1[n]
			}
			if colSets&ColSet2 != 0 {
				c2 = &info.C2[n]
			}
			if colSets&ColSet3 != 0 {
				c3 = &info.C3[n]
			}
			if !fn(nid, c1, c2, c3) {
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
	b.iterItems(ColSet1, func(nid int, c1 *Item1, c2 *Item2, c3 *Item3) bool {
		if _, ok := catStats[c1.Category]; !ok {
			catStats[c1.Category] = new(CatStat)
		}
		catStats[c1.Category].Items += 1
		catStats[c1.Category].Sales += c1.Sales
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
			b.iterItems(ColSet1|ColSet2, func(nid int, c1 *Item1, c2 *Item2, c3 *Item3) bool {
				if c1.Category != cat {
					return true
				}
				pt("%s\n", c2.Title)
				n++
				return n < 30
			})
			pt("\n")
		}
	}

	n := 0
	b.iterItems(0, func(nid int, c1 *Item1, c2 *Item2, c3 *Item3) bool {
		n++
		return true
	})
	pt("%d\n", n)

}

func (b *FileBackend) LogClient(info ClientInfo, state ClientState) {
}
