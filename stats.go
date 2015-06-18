package main

import "database/sql"

func stats(db *sql.DB, date string) {
	rows, err := db.Query(sp(`SELECT a.nid, cat, raw->'View_sales' FROM item_cats_%s a
		LEFT JOIN items_%s b ON a.nid = b.nid
	`, date, date))
	ce(err, "query")
	pt("query done")
	for rows.Next() {
		var nid uint64
		var cat uint64
		var salesStr string
		ce(rows.Scan(&nid, &cat, &salesStr), "scan row")
		pt("%d %d %s\n", nid, cat, salesStr)
	}
	ce(rows.Err(), "scan")
	rows.Close()

	//		item.View_sales = strings.Replace(item.View_sales, "人收货", "", -1)
	//		count, err := strconv.Atoi(item.View_sales)
	//		ce(err, sp("parse count %s", item.View_sales))
	//		price, err := strconv.ParseFloat(item.View_price, 64)
	//		ce(err, sp("parse price %s", item.View_price))
	//		amount := price * float64(count)
	//		/*
	//			if amount > 10000000 {
	//				pt("%s\n%d %f\n", item.Title, count, price)
	//				pt("http://item.taobao.com/item.htm?id=%s\n", item.Nid)
	//			}
	//		*/
	//		for _, cat := range item.Cats {
	//			if _, ok := catStats[cat]; !ok {
	//				catStats[cat] = &CatStat{
	//					Cat: cat,
	//				}
	//			}
	//			catStats[cat].Count += count
	//			catStats[cat].Amount += amount
	//		}
	//
	//
	//	for _, stat := range catStats {
	//		_, err = catStatsColle.Upsert(bson.M{"cat": stat.Cat}, stat)
	//		ce(err, "insert cat stat")
	//	}
	//
}
