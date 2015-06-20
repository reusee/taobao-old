package main

func (m *Mysql) checkSchema() (err error) {
	defer ct(&err)

	_, err = m.db.Exec(sp(`CREATE TABLE IF NOT EXISTS jobs_%s (
		cat BIGINT,
		page SMALLINT,
		done BOOL NOT NULL DEFAULT false,
		PRIMARY KEY (cat, page)
	) ENGINE = TokuDB`, m.date))
	ce(err, "create table")

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS cats (
		cat BIGINT PRIMARY KEY,
		name TEXT
	) ENGINE = TokuDB`)
	ce(err, "create table")

	_, err = m.db.Exec(`CREATE TABLE IF NOT EXISTS cat_relatives (
		cat BIGINT,
		rel BIGINT,
		PRIMARY KEY (cat, rel)
	) ENGINE = TokuDB`)

	return nil
}
