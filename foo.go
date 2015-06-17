package main

import (
	"bytes"
	"encoding/json"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func foo(db *mgo.Database) {
	now := time.Now()
	dateStr := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())

	/*
		itemsColle := db.C("items_" + dateStr)
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

	rawsColle := db.C("raws_" + dateStr)
	query := rawsColle.Find(bson.M{"cat": 50992010})
	iter := query.Iter()
	var raw Raw
	for iter.Next(&raw) {
		here := false
		for _, item := range raw.Items {
			if item.Nid == "39470054563" {
				here = true
			}
		}
		if here {
			pt("%d %d\n", raw.Page, raw.Cat)
			for _, item := range raw.Items {
				buf := new(bytes.Buffer)
				json.NewEncoder(buf).Encode(item)
				pt("%s\n", buf.Bytes())
			}
		}
	}

}
