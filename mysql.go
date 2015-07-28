package taobao

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var _ Backend = new(Mysql)

type Mysql struct {
	db         *sql.DB
	date       string
	date4mysql string
}

func NewMysql() (m *Mysql, err error) {
	defer ct(&err)

	db, err := sql.Open("mysql", "root@unix(/var/run/mysqld/mysqld.sock)/taobao?tokudb_commit_sync=off&parseTime=true")
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

func (m *Mysql) AddItems(items []Item, meta ItemsMeta) (err error) {
	defer ct(&err)
	for _, item := range items {
		//user
		_, err = m.db.Exec(`INSERT INTO users (id, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE name = ?`,
			item.Seller, item.SellerName, item.SellerName)
		ce(err, "insert user")
		//shop
		_, err = m.db.Exec(`INSERT INTO shops (id, is_tmall) VALUES (?, ?) ON DUPLICATE KEY UPDATE id=id`,
			item.SellerEncryptedId, item.SellerEncryptedId)
		ce(err, "insert shop")
		//item
		_, err = m.db.Exec(`INSERT INTO items (
			nid, title, 
			location, seller, shop) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE nid=nid`,
			item.Nid, item.Title,
			item.Location, item.Seller, item.SellerEncryptedId)
		ce(err, "insert item")
		//item stats
		price := item.Price.FloatString(3)
		_, err = m.db.Exec(`INSERT INTO item_stats (
			date, nid, price, sales, comments) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE price = ?, sales = ?, comments = ?`,
			m.date4mysql, item.Nid, price, item.Sales, item.Comments, price, item.Sales, item.Comments)
		ce(err, "insert item stats")
		//item cats
		_, err = m.db.Exec(`INSERT INTO item_fgcats (nid, cat)
			VALUES (?, ?) ON DUPLICATE KEY UPDATE nid=nid`, item.Nid, item.Category)
		ce(err, "insert item fgcats")
	}
	return
}

func (m *Mysql) AddFgCat(cat Cat) (err error) {
	tx, err := m.db.Begin()
	ce(err, "start transaction")
	_, err = tx.Exec(`INSERT INTO fgcats (cat, name) VALUES (?, ?) ON DUPLICATE KEY UPDATE name = ?`,
		cat.Cat, cat.Name, cat.Name)
	ce(err, "insert")
	for _, rel := range cat.Relatives {
		_, err = tx.Exec(`INSERT INTO fgcat_relatives (cat, rel) VALUES (?, ?) ON DUPLICATE KEY UPDATE cat=cat`,
			cat.Cat, rel)
		ce(err, "insert")
	}
	ce(tx.Commit(), "commit")
	return
}

func (m *Mysql) GetFgCats() (cats []Cat, err error) {
	defer ct(&err)
	rows, err := m.db.Query(`SELECT cat FROM fgcats`)
	ce(err, "query")
	for rows.Next() {
		var cat Cat
		ce(rows.Scan(&cat.Cat), "scan")
		cats = append(cats, cat)
	}
	ce(rows.Err(), "rows")
	return
}

func (m *Mysql) AddBgCat(cat Cat) (err error) {
	defer ct(&err)
	_, err = m.db.Exec(`INSERT INTO bgcats (cat, name, parent) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE cat=cat`,
		cat.Cat, cat.Name, cat.Parent)
	ce(err, "insert")
	return
}

func (m *Mysql) GetBgCatInfo(cat int) (info CatInfo, err error) {
	err = m.db.QueryRow(`SELECT last_checked FROM bgcats_info WHERE cat = ?`, cat).Scan(
		&info.LastChecked)
	switch {
	case err == sql.ErrNoRows:
		err = nil
	}
	return
}

func (m *Mysql) SetBgCatInfo(cat int, info CatInfo) (err error) {
	_, err = m.db.Exec(`REPLACE INTO bgcats_info (cat, last_checked) VALUES (?, ?)`,
		cat, info.LastChecked)
	return
}

func (m *Mysql) Stats() {
	_, err := m.db.Exec(`REPLACE INTO cat_stats (date, cat, sales)
		SELECT ?, cat, sum(sales) AS sales
		FROM item_stats a
		LEFT JOIN item_fgcats b
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
