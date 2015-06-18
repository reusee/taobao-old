package main

import "database/sql"

func checkSchema(db *sql.DB, date string) {
	_, err := db.Exec(sp(`CREATE TABLE IF NOT EXISTS jobs_%s (
		cat BIGINT,
		page SMALLINT,	
		done BOOLEAN NOT NULL DEFAULT false,
		PRIMARY KEY (cat, page)
	)`, date))
	ce(err, "create jobs table")
	_, err = db.Exec(sp(`CREATE INDEX done ON jobs_%s (done)`, date))
	ce(allowDupTable(err), "create index")
	_, err = db.Exec(sp(`CREATE INDEX page ON jobs_%s (page)`, date))
	ce(allowDupTable(err), "create index")

	_, err = db.Exec(sp(`CREATE TABLE IF NOT EXISTS items_%s (
		nid BIGINT PRIMARY KEY,
		raw JSONB
	)`, date))
	ce(err, "create items table")

	_, err = db.Exec(sp(`CREATE TABLE IF NOT EXISTS htmls_%s (
		cat BIGINT,
		page SMALLINT,
		html bytea,
		PRIMARY KEY (cat, page)
	)`, date))
	ce(err, "create htmls table")

	_, err = db.Exec(sp(`CREATE TABLE IF NOT EXISTS item_cats_%s (
		nid BIGINT,
		cat BIGINT,
		PRIMARY KEY (nid, cat)
	)`, date))
	ce(err, "create item cats table")
}
