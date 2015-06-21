package main

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Mysql struct {
	db         *sql.DB
	date       string
	date4mysql string
}

func NewMysql() (m *Mysql, err error) {
	defer ct(&err)

	db, err := sql.Open("mysql", "root@unix(/var/run/mysqld/mysqld.sock)/taobao?tokudb_commit_sync=off")
	ce(err, "open sql connection")

	now := time.Now()
	date := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())
	date4mysql := sp("%04d-%02d-%02d", now.Year(), now.Month(), now.Day())

	m = &Mysql{
		db:         db,
		date:       date,
		date4mysql: date4mysql,
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
		_, err := tx.Exec(sp(`INSERT INTO jobs_%s (cat, page) VALUES (?, ?) ON DUPLICATE KEY UPDATE cat=cat`, m.date),
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
	for _, item := range items {
		ce(err, "start transaction")
		//user
		uid, err := strconv.Atoi(item.User_id)
		ce(err, sp("parse uid %s", item.User_id))
		_, err = m.db.Exec(`INSERT INTO users (id, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE name = ?`,
			uid, item.Nick, item.Nick)
		ce(err, "insert user")
		//shop
		_, err = m.db.Exec(`INSERT INTO shops (id, is_tmall) VALUES (?, ?) ON DUPLICATE KEY UPDATE id=id`,
			item.Shopcard.EncryptedUserId, item.Shopcard.IsTmall)
		ce(err, "insert shop")
		//item
		nid, err := strconv.Atoi(item.Nid)
		ce(err, sp("parse nid %s", item.Nid))
		_, err = m.db.Exec(`INSERT INTO items (
			nid, title, raw_title, pic_url, detail_url, comment_url, 
			location, seller, shop) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE nid=nid`,
			nid, item.Title, item.Raw_title, item.Pic_url, item.Detail_url, item.Comment_url,
			item.Item_loc, uid, item.Shopcard.EncryptedUserId)
		ce(err, "insert item")
		//item stats
		price, err := strconv.ParseFloat(item.View_price, 64)
		ce(err, sp("parse price %s", item.View_price))
		salesStr := item.View_sales
		salesStr = strings.Replace(salesStr, "人收货", "", -1)
		salesStr = strings.Replace(salesStr, "人付款", "", -1)
		sales, err := strconv.Atoi(salesStr)
		ce(err, sp("parse sales %s", item.View_sales))
		comments := 0
		if len(item.Comment_count) > 0 {
			comments, err = strconv.Atoi(item.Comment_count)
		}
		ce(err, sp("parse comment count %s", item.Comment_count))
		_, err = m.db.Exec(`INSERT INTO item_stats (
			date, nid, price, sales, comments) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE price = ?, sales = ?, comments = ?`,
			m.date4mysql, nid, price, sales, comments, price, sales, comments)
		ce(err, "insert item stats")
		//item sources
		_, err = m.db.Exec(`INSERT INTO item_sources (date, nid, cat, page)
			VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE date=date`, m.date4mysql, nid, job.Cat, job.Page)
		ce(err, "insert item sources")
		_, err = m.db.Exec(`INSERT INTO item_cats (nid, cat)
			VALUES (?, ?) ON DUPLICATE KEY UPDATE nid=nid`, nid, job.Cat)
		ce(err, "insert item cats")
	}
	return
}

func (m *Mysql) AddCat(cat Cat) (err error) {
	tx, err := m.db.Begin()
	ce(err, "start transaction")
	_, err = tx.Exec(`INSERT INTO cats (cat, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE name = ?`,
		cat.Cat, cat.Name, cat.Name)
	ce(err, "insert")
	for _, rel := range cat.Relatives {
		_, err = tx.Exec(`INSERT INTO cat_relatives (cat, rel) VALUES (?, ?) ON DUPLICATE KEY UPDATE cat=cat`,
			cat.Cat, rel)
		ce(err, "insert")
	}
	ce(tx.Commit(), "commit")
	return
}

func (m *Mysql) GetCats() (cats []Cat, err error) {
	defer ct(&err)
	rows, err := m.db.Query(`SELECT cat FROM cats`)
	ce(err, "query")
	for rows.Next() {
		var cat Cat
		ce(rows.Scan(&cat.Cat), "scan")
		cats = append(cats, cat)
	}
	ce(rows.Err(), "rows")
	return
}

func (m *Mysql) Stats() {
	_, err := m.db.Exec(`REPLACE INTO cat_stats (date, cat, sales)
		SELECT ?, cat, sum(sales) AS sales
		FROM item_stats a
		LEFT JOIN item_cats b
		ON a.nid=b.nid
		WHERE date = ?
		GROUP BY cat
	`, m.date4mysql, m.date4mysql)
	ce(err, "update category sales")
}

func (m *Mysql) LogClient(info ClientInfo, state ClientState) {
	if info.HttpProxyAddr == "" {
		return
	}
	switch state {
	case Good:
		_, err := m.db.Exec(`INSERT INTO proxies (date, addr, good) VALUES
		(?, ?, 1) ON DUPLICATE KEY UPDATE good=good+1`, m.date4mysql, info.HttpProxyAddr)
		ce(err, "insert proxy log")
	case Bad:
		_, err := m.db.Exec(`INSERT INTO proxies (date, addr, bad) VALUES
		(?, ?, 1) ON DUPLICATE KEY UPDATE bad=bad+1`, m.date4mysql, info.HttpProxyAddr)
		ce(err, "insert proxy log")
	}
}
