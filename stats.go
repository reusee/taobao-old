package main

import (
	"strconv"
	"strings"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func stats(db *mgo.Database) {
	dateStr := "20150617"
	itemsColle := db.C("items_" + dateStr)

	catStatsColle := db.C("cat_stats_" + dateStr)
	err := catStatsColle.Create(&mgo.CollectionInfo{
		Extra: bson.M{
			"compression": "zlib",
		},
	})
	ce(ignoreExistsColle(err), "create cat_stats")
	err = catStatsColle.EnsureIndex(mgo.Index{
		Key:    []string{"cat"},
		Unique: true,
		Sparse: true,
	})
	type CatStat struct {
		Cat    int
		Count  int
		Amount float64
	}
	catStats := make(map[int]*CatStat)

	query := itemsColle.Find(nil)
	iter := query.Iter()
	var item Item
	for iter.Next(&item) {
		item.View_sales = strings.Replace(item.View_sales, "人收货", "", -1)
		count, err := strconv.Atoi(item.View_sales)
		ce(err, sp("parse count %s", item.View_sales))
		price, err := strconv.ParseFloat(item.View_price, 64)
		ce(err, sp("parse price %s", item.View_price))
		amount := price * float64(count)
		/*
			if amount > 10000000 {
				pt("%s\n%d %f\n", item.Title, count, price)
				pt("http://item.taobao.com/item.htm?id=%s\n", item.Nid)
			}
		*/
		for _, cat := range item.Cats {
			if _, ok := catStats[cat]; !ok {
				catStats[cat] = &CatStat{
					Cat: cat,
				}
			}
			catStats[cat].Count += count
			catStats[cat].Amount += amount
		}

	}

	for _, stat := range catStats {
		_, err = catStatsColle.Upsert(bson.M{"cat": stat.Cat}, stat)
		ce(err, "insert cat stat")
	}

}
