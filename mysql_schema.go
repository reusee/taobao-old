package main

func (m *Mysql) checkSchema() (err error) {
	defer ct(&err)

	_, err = m.db.Exec(sp(`CREATE TABLE IF NOT EXISTS jobs_%s (
		cat BIGINT,
		page SMALLINT,
		done BOOL NOT NULL DEFAULT false,
		PRIMARY KEY (cat, page)
	) ENGINE = InnoDB`, m.date))
	ce(err, "create jobs table")

	return nil
}
