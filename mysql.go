package main

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Mysql struct {
	db   *sql.DB
	date string
}

func NewMysql() (m *Mysql, err error) {
	defer ct(&err)

	db, err := sql.Open("mysql", "root@unix(/var/run/mysqld/mysqld.sock)/taobao")
	ce(err, "open sql connection")

	now := time.Now()
	date := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())

	m = &Mysql{
		db:   db,
		date: date,
	}

	ce(m.checkSchema(), "check schema")

	return m, nil
}

func (m *Mysql) Close() {
	m.db.Close()
}

func (m *Mysql) AddJobs(jobs []Job) (err error) {
	defer ct(&err)
	tx, err := m.db.Begin()
	ce(err, "start transaction")
	for _, job := range jobs {
		_, err := tx.Exec(sp(`INSERT IGNORE INTO jobs_%s (cat, page) VALUES (?, ?)`, m.date),
			job.Cat, job.Page)
		ce(err, "add job")
	}
	ce(tx.Commit(), "commit")
	return
}

func (m *Mysql) DoneJob(job Job) error {
	_, err := m.db.Exec(sp(`UPDATE jobs_%s SET done = true WHERE cat = ? AND page = ? LIMIT 1`, m.date),
		job.Cat, job.Page)
	return err
}

func (m *Mysql) GetJobs() (jobs []Job, err error) {
	defer ct(&err)
	rows, err := m.db.Query(sp(`SELECT cat, page FROM jobs_%s WHERE done = false`, m.date))
	ce(err, "query")
	for rows.Next() {
		var job Job
		err = rows.Scan(&job.Cat, &job.Page)
		ce(err, "scan")
		jobs = append(jobs, job)
	}
	err = rows.Err()
	ce(err, "get rows")
	return jobs, nil
}

func (m *Mysql) AddItems(items []Item, job Job) (err error) {
	defer ct(&err)
	tx, err := m.db.Begin()
	ce(err, "start transaction")
	for _, item := range items {
		_ = item
		//TODO
	}
	ce(tx.Commit(), "commit")
	return
}

func (m *Mysql) AddCat(cat Cat) error {
	return nil //TODO
}

func (m *Mysql) Stats() {
	//TODO
}
