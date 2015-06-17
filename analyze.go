package main

import (
	"strconv"
	"strings"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func analyze(db *mgo.Database) {
	dateStr := "20150616"
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

		cat, err := strconv.Atoi(item.Category)
		ce(err, "parse category")
		if _, ok := catStats[cat]; !ok {
			catStats[cat] = &CatStat{
				Cat: cat,
			}
		}
		item.View_sales = strings.Replace(item.View_sales, "人收货", "", -1)
		count, err := strconv.Atoi(item.View_sales)
		ce(err, sp("parse count %s", item.View_sales))
		catStats[cat].Count += count
		price, err := strconv.ParseFloat(item.View_price, 64)
		ce(err, sp("parse price %s", item.View_price))
		amount := price * float64(count)
		catStats[cat].Amount += amount

	}

	for _, stat := range catStats {
		err = catStatsColle.Insert(stat)
		ce(allowDup(err), "insert cat stat")
	}

}
