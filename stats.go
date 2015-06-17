package main

import (
	"strconv"
	"strings"
	"time"

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
	catStatsColle.RemoveAll(nil)
	type CatStat struct {
		Cat    int
		Count  int
		Amount float64
	}
	catStats := make(map[int]*CatStat)

	itemCatsColle := db.C("item_cats_" + dateStr)

	query := itemsColle.Find(nil)
	iter := query.Iter()
	var item Item
	t0 := time.Now()
	n := 0
	for iter.Next(&item) {
		n++
		if n%30000 == 0 {
			pt("%10d %v\n", n, time.Now().Sub(t0))
		}

		nid, err := strconv.Atoi(item.Nid)
		ce(err, sp("parse nid %s", item.Nid))
		item.View_sales = strings.Replace(item.View_sales, "人收货", "", -1)
		count, err := strconv.Atoi(item.View_sales)
		ce(err, sp("parse count %s", item.View_sales))
		price, err := strconv.ParseFloat(item.View_price, 64)
		ce(err, sp("parse price %s", item.View_price))
		amount := price * float64(count)
		var itemCats []ItemCat
		err = itemCatsColle.Find(bson.M{"nid": nid}).All(&itemCats)
		ce(err, "get item cats")
		for _, itemCat := range itemCats {
			if _, ok := catStats[itemCat.Cat]; !ok {
				catStats[itemCat.Cat] = &CatStat{
					Cat: itemCat.Cat,
				}
			}
			catStats[itemCat.Cat].Count += count
			catStats[itemCat.Cat].Amount += amount
		}

	}

	for _, stat := range catStats {
		_, err = catStatsColle.Upsert(bson.M{"cat": stat.Cat}, stat)
		ce(err, "insert cat stat")
	}

}
