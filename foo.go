package main

import "gopkg.in/mgo.v2"

func foo(db *mgo.Database) {
	dateStr := "20150617"
	itemsColle := db.C("items_" + dateStr)

	query := itemsColle.Find(nil)
	iter := query.Iter()
	var item Item
	for iter.Next(&item) {
		for _, cat := range item.Cats {
			if cat == 50043648 {
				pt("%s %s\n", item.View_sales, item.Nid)
			}
		}
	}

}
