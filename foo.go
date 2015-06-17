package main

import (
	"time"

	"gopkg.in/mgo.v2"
)

func foo(db *mgo.Database) {
	now := time.Now()
	dateStr := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())
	itemsColle := db.C("items_" + dateStr)

	query := itemsColle.Find(nil)
	iter := query.Iter()
	var item Item
	for iter.Next(&item) {
		for _, cat := range item.Cats {
			if cat == 55070016 {
				pt("%s %s\n", item.View_sales, item.Nid)
			}
		}
	}

}
