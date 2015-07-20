package main

import (
	"path/filepath"
	"strconv"

	"github.com/reusee/jsonfile"
)

type FileBackend struct {
	fgCats     StrSet
	fgCatsFile *jsonfile.File
}

func NewFileBackend() (b *FileBackend, err error) {
	defer ct(&err)
	//now := time.Now()
	//date := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())
	dataDir := "data"

	b = &FileBackend{
		fgCats: NewStrSet(),
	}
	b.fgCatsFile, err = jsonfile.New(&b.fgCats, filepath.Join(dataDir, "fgcats"),
		jsonfile.NewFileLocker(filepath.Join(dataDir, "fgcats.lock")))
	ce(err, "fgcats file")

	return b, nil
}

func (b *FileBackend) Close() {
	b.fgCatsFile.Close()
}

func (b *FileBackend) AddBgCat(cat Cat) error {
	panic("not needed")
	return nil
}

func (b *FileBackend) GetBgCatInfo(cat int) (info CatInfo, err error) {
	panic("not needed")
	return
}

func (b *FileBackend) SetBgCatInfo(cat int, info CatInfo) error {
	panic("not needed")
	return nil
}

func (b *FileBackend) AddFgCat(cat Cat) (err error) {
	b.fgCats.Add(strconv.Itoa(cat.Cat))
	err = b.fgCatsFile.Save()
	ce(err, "save fgcats file")
	//TODO
	return nil
}

func (b *FileBackend) GetFgCats() (cats []Cat, err error) {
	//TODO
	return
}

func (b *FileBackend) AddItems(items []Item, job Job) error {
	//TODO
	return nil
}

func (b *FileBackend) AddJobs(jobs []Job) error {
	//TODO
	return nil
}

func (b *FileBackend) GetJobs() (jobs []Job, err error) {
	//TODO
	return
}

func (b *FileBackend) DoneJob(job Job) error {
	//TODO
	return nil
}

func (b *FileBackend) Foo() {
}

func (b *FileBackend) Stats() {
}

func (b *FileBackend) LogClient(info ClientInfo, state ClientState) {
}
