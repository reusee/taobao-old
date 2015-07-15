package main

func (m *Mysql) checkSchema() (err error) {
	defer ct(&err)

	_, err = m.db.Exec(sp(`CREATE TABLE IF NOT EXISTS jobs_%s (
		cat BIGINT REFERENCES cats(cat),
		page SMALLINT,
		done BOOL NOT NULL DEFAULT false,
		PRIMARY KEY (cat, page)
	) ENGINE = TokuDB`, m.date))
	ce(err, "create table jobs")

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS cats (
		cat BIGINT PRIMARY KEY,
		name TEXT
	) ENGINE = TokuDB`)
	ce(err, "create table cats")

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS cat_relatives (
		cat BIGINT REFERENCES cats(cat),
		rel BIGINT REFERENCES cats(cat),
		PRIMARY KEY (cat, rel),
		INDEX (rel)
	) ENGINE = TokuDB`)
	ce(err, "create table cat_relatives")

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS items (
		nid BIGINT PRIMARY KEY,
		title TEXT,
		pic_url TEXT,
		detail_url TEXT,
		comment_url TEXT,
		location TEXT,
		seller BIGINT REFERENCES users(id),
		shop CHAR(32) REFERENCES shops(id),
		INDEX (seller),
		INDEX (shop)
	) ENGINE = TokuDB`)
	ce(err, "create table items")

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS item_stats (
		date DATE,
		nid BIGINT REFERENCES items(nid),
		price DECIMAL(12, 2),
		sales BIGINT,
		comments BIGINT,
		PRIMARY KEY (date, nid),
		INDEX (nid),
		INDEX (price),
		INDEX (sales),
		INDEX (comments)
	) ENGINE = TokuDB`)
	ce(err, "create table item_stats")

	/*
		_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS item_sources (
			date DATE,
			nid BIGINT REFERENCES items(nid),
			cat BIGINT REFERENCES cats(cat),
			page SMALLINT,
			PRIMARY KEY (date, nid, cat, page),
			INDEX (nid),
			INDEX (cat)
		) ENGINE = TokuDB`)
		ce(err, "create table item_sources")
	*/

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS item_cats (
		nid BIGINT REFERENCES items(nid),
		cat BIGINT REFERENCES cats(cat),
		PRIMARY KEY (nid, cat),
		INDEX (cat)
	) ENGINE = TokuDB`)
	ce(err, "create table item_cats")

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id BIGINT PRIMARY KEY,
		name TEXT
	) ENGINE = TokuDB`)
	ce(err, "create table users")

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS shops (
		id CHAR(32) PRIMARY KEY,
		is_tmall BOOL,
		level SMALLINT,
		INDEX (is_tmall),
		INDEX (level)
	) ENGINE = TokuDB`)
	ce(err, "create table shops")

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS cat_stats (
		date DATE,
		cat BIGINT REFERENCES cats(cat),
		sales BIGINT,
		PRIMARY KEY (date, cat),
		INDEX (cat),
		INDEX (sales)
	) ENGINE = TokuDB`)
	ce(err, "create table cat_stats")

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS proxies (
		date DATE,
		addr CHAR(32),
		good INTEGER DEFAULT 0,
		bad INTEGER DEFAULT 0,
		PRIMARY KEY (date, addr)
	) ENGINE = TokuDB`)
	ce(err, "create table proxies")

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS bgcats (
		cat BIGINT PRIMARY KEY,
		parent BIGINT REFERENCES cat,
		name TEXT
	) ENGINE = TokuDB`)
	ce(err, "create background cats table")

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS bgcats_info (
		cat BIGINT PRIMARY KEY,
		last_checked DATETIME
	) ENGINE = TokuDB`)
	ce(err, "create background cats info table")

	return nil
}
