package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/gob"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/reusee/gobfile"
	"github.com/reusee/jsonfile"
)

type FileBackend struct {
	fgCats     StrSet
	fgCatsFile *jsonfile.File

	jobs     map[Job]bool
	jobsLock *sync.Mutex
	jobsFile *gobfile.File

	itemsFile *os.File
	itemsLock *sync.Mutex

	closed chan struct{}
}

func NewFileBackend() (b *FileBackend, err error) {
	defer ct(&err)
	now := time.Now()
	date := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())
	dataDir := "data"

	b = &FileBackend{
		fgCats: NewStrSet(),
		closed: make(chan struct{}),
	}

	b.fgCatsFile, err = jsonfile.New(&b.fgCats, filepath.Join(dataDir, "fgcats"),
		jsonfile.NewFileLocker(filepath.Join(dataDir, "fgcats.lock")))
	ce(err, "fgcats file")

	b.jobs = make(map[Job]bool)
	b.jobsLock = new(sync.Mutex)
	b.jobsFile, err = gobfile.New(&b.jobs, filepath.Join(dataDir, sp("%s-jobs", date)),
		gobfile.NewFileLocker(filepath.Join(dataDir, sp("%s-jobs.lock", date))))
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
	pt("checking items file\n")
	for {
		var l uint32
		err = binary.Read(itemsFile, binary.LittleEndian, &l)
		if err == io.EOF {
			break
		}
		ce(err, "read entry len")
		_, err = itemsFile.Seek(int64(l), os.SEEK_CUR)
		ce(err, "seek entry")
	}
	pt("items file is good\n")
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
	err = gob.NewEncoder(w).Encode(items)
	ce(err, "gob encode")
	err = w.Close()
	ce(err, "close write")
	bs := buf.Bytes()
	withLock(b.itemsLock, func() {
		err = binary.Write(b.itemsFile, binary.LittleEndian, uint32(len(bs)))
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
	withLock(b.jobsLock, func() {
		job := Job{
			Cat:  job.Cat,
			Page: job.Page,
		}
		b.jobs[job] = true
	})
	return nil
}

func (b *FileBackend) Foo() {
}

func (b *FileBackend) Stats() {
}

func (b *FileBackend) LogClient(info ClientInfo, state ClientState) {
}
