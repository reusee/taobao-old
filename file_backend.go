package taobao

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/reusee/dsfile"
	"github.com/ugorji/go/codec"
)

var codecHandle = new(codec.CborHandle)

type FileBackend struct {
	fgCats     StrSet
	fgCatsFile *dsfile.File

	jobs     map[Job]bool
	jobsLock *sync.Mutex
	jobsFile *dsfile.File

	itemsFile *os.File
	itemsLock *sync.Mutex

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
		fgCats: NewStrSet(),
		closed: make(chan struct{}),
	}

	b.fgCatsFile, err = dsfile.New(&b.fgCats, filepath.Join(dataDir, "fgcats"),
		new(dsfile.Json), dsfile.NewFileLocker(filepath.Join(dataDir, "fgcats.lock")))
	ce(err, "fgcats file")

	b.jobs = make(map[Job]bool)
	b.jobsLock = new(sync.Mutex)
	b.jobsFile, err = dsfile.New(&b.jobs, filepath.Join(dataDir, sp("%s-jobs", date)),
		new(dsfile.Gob), dsfile.NewFileLocker(filepath.Join(dataDir, sp("%s-jobs.lock", date))))
	ce(err, "jobs file")
	go func() {
		t := time.NewTicker(time.Second * 3)
		for {
			select {
			case <-t.C:
				err := b.jobsFile.Save()
				ce(err, "save jobs file")
			case <-b.closed:
				return
			}
		}
	}()

	b.itemsLock = new(sync.Mutex)
	itemsFile, err := os.OpenFile(filepath.Join(dataDir, sp("%s-items", date)),
		os.O_RDWR|os.O_CREATE, 0644)
	ce(err, "open items file")
	b.itemsFile = itemsFile

	err = b.scanItemsFile()
	ce(err, "scan items file")

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
		b.jobs[job] = true
		doneJobs++
	}
	pt("items file is good, %d jobs done\n", doneJobs)
	return
}

func (b *FileBackend) Close() {
	b.fgCatsFile.Close()
	b.jobsFile.Close()
	b.itemsFile.Close()
}

func (b *FileBackend) AddBgCat(cat Cat) error {
	if true {
		panic("not needed")
	}
	return nil
}

func (b *FileBackend) GetBgCatInfo(cat int) (info CatInfo, err error) {
	if true {
		panic("not needed")
	}
	return
}

func (b *FileBackend) SetBgCatInfo(cat int, info CatInfo) error {
	if true {
		panic("not needed")
	}
	return nil
}

func (b *FileBackend) AddFgCat(cat Cat) (err error) {
	b.fgCats.Add(strconv.Itoa(cat.Cat))
	err = b.fgCatsFile.Save()
	ce(err, "save fgcats file")
	return nil
}

func (b *FileBackend) GetFgCats() (cats []Cat, err error) {
	for cat := range b.fgCats {
		c, err := strconv.Atoi(cat)
		ce(err, "strconv")
		cats = append(cats, Cat{
			Cat: c,
		})
	}
	return
}

func (b *FileBackend) AddItems(items []Item, job Job) (err error) {
	defer ct(&err)
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

func (b *FileBackend) AddJobs(jobs []Job) error {
	for _, job := range jobs {
		job = Job{ // make it a key
			Cat:  job.Cat,
			Page: job.Page,
		}
		withLock(b.jobsLock, func() {
			if _, ok := b.jobs[job]; ok { // exists
				return
			}
			b.jobs[job] = false
		})
	}
	return nil
}

func (b *FileBackend) GetJobs() (jobs []Job, err error) {
	defer ct(&err)
	err = b.scanItemsFile()
	ce(err, "scan items file")
	withLock(b.jobsLock, func() {
		for job, done := range b.jobs {
			if done {
				continue
			}
			jobs = append(jobs, job)
		}
	})
	return
}

func (b *FileBackend) DoneJob(job Job) error {
	return nil
}

func (b *FileBackend) Foo() {
	b.itemsFile.Seek(0, os.SEEK_SET)
	n := 0
	t0 := time.Now()
	for {
		var header EntryHeader
		err := binary.Read(b.itemsFile, binary.LittleEndian, &header)
		if err == io.EOF {
			err = nil
			break
		}
		ce(err, "read length")
		bs := make([]byte, header.Len)
		_, err = io.ReadFull(b.itemsFile, bs)
		ce(err, "read data")
		r, err := gzip.NewReader(bytes.NewReader(bs))
		ce(err, "new gzip reader")
		var items []Item
		err = codec.NewDecoder(r, codecHandle).Decode(&items)
		ce(err, "decode")
		n++
		if n%1000 == 0 {
			pt("%d %v\n", n, time.Now().Sub(t0))
		}
	}
}

func (b *FileBackend) Stats() {
}

func (b *FileBackend) LogClient(info ClientInfo, state ClientState) {
}
