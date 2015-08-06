package taobao

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/reusee/dsfile"
	"github.com/reusee/rcf"
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

	itemsFile           *rcf.File
	collectDoneJobsOnce sync.Once

	closed chan struct{}
}

func NewFileBackend(now time.Time) (b *FileBackend, err error) {
	defer ct(&err)
	date := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())
	dataDir := "data"

	b = &FileBackend{
		date:      date,
		dataDir:   dataDir,
		fgCats:    NewIntSet(),
		bgCats:    make(map[int]*BgCat),
		closed:    make(chan struct{}),
		collected: make(map[Job]bool),
	}

	b.fgCatsFile, err = dsfile.New(&b.fgCats, filepath.Join(dataDir, "fgcats"),
		new(dsfile.Cbor), dsfile.NewFileLocker(filepath.Join(dataDir, "fgcats.lock")))
	ce(err, "fgcats file")

	b.bgCatsFile, err = dsfile.New(&b.bgCats, filepath.Join(dataDir, "bgcats"),
		new(dsfile.Cbor), dsfile.NewFileLocker(filepath.Join(dataDir, "bgcats.lock")))
	ce(err, "bgcats file")

	itemsFile, err := rcf.New(filepath.Join(dataDir, sp("%s-items.rcf", date)),
		func(i int) interface{} {
			switch i {
			case 0:
				return &struct {
					Nid []int
				}{}
			case 1:
				return &struct {
					Category []int
					Price    []*big.Rat
					Sales    []int
					Seller   []int
				}{}
			case 2:
				return &struct {
					Title    []string
					Location []string
				}{}
			case 3:
				return &struct {
					Comments          []int
					SellerEncryptedId []string
					SellerName        []string
					SellerLevels      [][]uint8
					SellerIsTmall     []bool
					SellerCredit      []int
				}{}
			}
			return nil
		})
	ce(err, "open items file")
	b.itemsFile = itemsFile

	go func() {
		t := time.NewTicker(time.Second*3 + time.Millisecond*500)
		for {
			select {
			case <-t.C:
				err := itemsFile.Sync()
				ce(err, "sync items file")
			case <-b.closed:
				return
			}
		}
	}()

	return b, nil
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
	return b.itemsFile.Append(items, job)
}

func (b *FileBackend) IsCollected(job Job) bool {
	b.collectDoneJobsOnce.Do(func() {
		n := 0
		b.itemsFile.IterMetas(func(job Job) bool {
			job = Job{
				Cat:  job.Cat,
				Page: job.Page,
			}
			b.collected[job] = true
			n++
			return true
		})
		pt("%d jobs done\n", n)
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

func (b *FileBackend) PostProcess() {
	t0 := time.Now()
	defer func() {
		pt("post processed in %v\n", time.Now().Sub(t0))
	}()

	// cat stats
	catStats := make(map[int]*CatStat)
	b.itemsFile.Iter([]string{"Category", "Sales"}, func(cols ...interface{}) bool {
		cats := cols[0].([]int)
		sales := cols[1].([]int)
		for n, cat := range cats {
			if _, ok := catStats[cat]; !ok {
				catStats[cat] = new(CatStat)
			}
			catStats[cat].Items += 1
			catStats[cat].Sales += sales[n]
		}
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
			b.itemsFile.Iter([]string{"Category", "Title"}, func(cols ...interface{}) bool {
				cats := cols[0].([]int)
				titles := cols[1].([]string)
				for i, category := range cats {
					if category != cat {
						continue
					}
					pt("%s\n", titles[i])
					n++
				}
				return n < 30
			})
			pt("\n")
		}
	}

	n := 0
	b.itemsFile.Iter([]string{"Nid"}, func(cols ...interface{}) bool {
		nids := cols[0].([]int)
		n += len(nids)
		return true
	})
	pt("%d\n", n)

}

func (b *FileBackend) LogClient(info ClientInfo, state ClientState) {
}
