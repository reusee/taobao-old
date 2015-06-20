package main

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type Mysql struct {
	db *sql.DB
}

func NewMysql() (m *Mysql, err error) {
	defer ct(&err)

	db, err := sql.Open("mysql", "root@/taobao")
	ce(err, "open sql connection")

	return &Mysql{
		db: db,
	}, nil
}

func (m *Mysql) Close() {
	m.db.Close()
}

func (m *Mysql) AddJob(job Job) error {
	return nil //TODO
}

func (m *Mysql) DoneJob(job Job) error {
	return nil //TODO
}

func (m *Mysql) GetJobs() ([]Job, error) {
	return nil, nil //TODO
}

func (m *Mysql) AddItem(item Item, job Job) error {
	return nil //TODO
}

func (m *Mysql) AddCat(cat Cat) error {
	return nil //TODO
}

func (m *Mysql) Stats() {
	//TODO
}
