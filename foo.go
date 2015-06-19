package main

import (
	"strconv"
	"strings"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func foo(db *mgo.Database, date string) {

	/*
		itemsColle := db.C("items_" + date)
		query := itemsColle.Find(nil)
		iter := query.Iter()
		var item Item
		for iter.Next(&item) {
			for _, cat := range item.Cats {
				if cat == 50992010 {
					pt("%s %s\n", item.View_sales, item.Nid)
				}
			}
		}
	*/

	rawsColle := db.C("raws_" + date)
	query := rawsColle.Find(bson.M{"cat": 50390004})
	iter := query.Iter()
	var raw Raw
	n := 0
	for iter.Next(&raw) {
		for _, item := range raw.Items {
			pt("%s\n", item.Title)
			pt("%s\n", item.View_sales)
			pt("%s\n", item.View_price)
			item.View_sales = strings.Replace(item.View_sales, "人收货", "", -1)
			count, err := strconv.Atoi(item.View_sales)
			ce(err, sp("parse count %s", item.View_sales))
			n += count
		}
		pt("\n")
	}
	pt("%d\n", n)

}
