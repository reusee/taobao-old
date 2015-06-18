package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/reusee/hcutil"
)

func collect(db *sql.DB, date string) {
	// client provider
	clientsIn, clientsOut, killClientsChan := ClientsProvider()
	defer close(killClientsChan)

	type Job struct {
		Cat, Page int
		Done      bool
	}

	// first-page jobs
	content, err := ioutil.ReadFile("categories")
	ce(err, "read categories file")
	pt("start insert first-page jobs\n")
	for _, line := range bytes.Split(content, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		catStr := line[bytes.LastIndex(line, []byte(" "))+1:]
		cat, err := strconv.Atoi(string(catStr))
		ce(err, "parse cat id")
		_, err = db.Exec(sp(`INSERT INTO jobs_%s (cat, page) VALUES ($1, 0)`, date),
			cat)
		ce(allowUniqVio(err), "insert job")
	}
	pt("first-page jobs inserted\n")

	markDone := func(cat, page int) {
		_, err := db.Exec(sp(`UPDATE jobs_%s SET done = true WHERE cat = $1 AND page = $2`, date),
			cat, page)
		ce(err, "mark done")
	}
	_ = markDone

	// status
	var itemsCount uint64
	var jobsTotal int64
	var jobsDone int64
	go func() {
		for range time.NewTicker(time.Second * 3).C {
			pt("%d / %d jobs done. %d items collected\n",
				atomic.LoadInt64(&jobsDone),
				jobsTotal,
				atomic.LoadUint64(&itemsCount))
		}
	}()

	// collect
collect:
	jobs := []Job{}
	rows, err := db.Query(sp(`SELECT cat, page FROM jobs_%s WHERE done = false`, date))
	ce(err, "query")
	for rows.Next() {
		var job Job
		ce(rows.Scan(&job.Cat, &job.Page), "scan")
		jobs = append(jobs, job)
	}
	ce(rows.Err(), "get rows")
	rows.Close()
	pt("%d jobs\n", len(jobs))
	if len(jobs) == 0 {
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(jobs))
	jobsTotal = int64(len(jobs))
	jobsDone = 0
	sem := make(chan struct{}, 256)
	for _, job := range jobs {
		client := <-clientsOut
		job := job
		sem <- struct{}{}
		go func() {
			defer func() {
				wg.Done()
				atomic.AddInt64(&jobsDone, 1)
				<-sem
			}()
			url := sp("http://s.taobao.com/list?cat=%d&sort=sale-desc&bcoffset=0&s=%d", job.Cat, job.Page*60)
			bs, err := hcutil.GetBytes(client, url)
			if err != nil {
				pt(sp("get %s error: %v\n", url, err))
				return
			}
			jstr, err := GetPageConfigJson(bs)
			if err != nil {
				pt(sp("get %s page config error: %v\n", url, err))
				return
			}
			var config PageConfig
			err = json.Unmarshal(jstr, &config)
			if err != nil {
				pt(sp("unmarshal %s json error: %v\n", url, err))
				return
			}
			if config.Mods["itemlist"].Status == "hide" { // no items
				markDone(job.Cat, job.Page)
				clientsIn <- client
				return
			}
			items, err := GetItems(config.Mods["itemlist"].Data)
			if err != nil {
				pt(sp("unmarshal item list %s error: %v\n", url, err))
				return
			}
			for _, item := range items {
				nid, err := strconv.Atoi(item.Nid)
				ce(err, sp("parse nid %s", item.Nid))
				jsonBs, err := json.Marshal(item)
				ce(err, "marshal item")
				_, err = db.Exec(sp(`INSERT INTO items_%s (nid, raw) VALUES ($1, $2)`, date),
					nid, jsonBs)
				ce(allowUniqVio(err), "insert item")
				_, err = db.Exec(sp(`INSERT INTO item_cats_%s (nid, cat) VALUES ($1, $2)`, date),
					nid, job.Cat)
				ce(allowUniqVio(err), "insert item cat")
			}
			_, err = db.Exec(sp(`INSERT INTO htmls_%s (cat, page, html) VALUES ($1, $2, $3)`, date),
				job.Cat, job.Page, bs)
			ce(allowUniqVio(err), "insert html")
			atomic.AddUint64(&itemsCount, uint64(len(items)))
			if config.Mods["pager"].Status == "hide" || job.Page > 0 {
				markDone(job.Cat, job.Page)
				clientsIn <- client
				return
			}
			var pagerData struct {
				TotalPage int
			}
			err = json.Unmarshal(config.Mods["pager"].Data, &pagerData)
			if err != nil {
				pt(sp("unmarshal pager %s error: %v\n", url, err))
				return
			}
			maxPage := 10
			if pagerData.TotalPage < maxPage {
				maxPage = pagerData.TotalPage
			}
			for i := 1; i < maxPage; i++ {
				_, err = db.Exec(sp(`INSERT INTO jobs_%s (cat, page) VALUES ($1, $2)`, date),
					job.Cat, i)
				ce(allowUniqVio(err), "insert job")
			}
			markDone(job.Cat, job.Page)
			clientsIn <- client
		}()
	}
	wg.Wait()
	goto collect

}

func GetPageConfigJson(content []byte) ([]byte, error) {
	var jStr []byte
	for _, line := range bytes.Split(content, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		line = bytes.TrimLeft(line, " ")
		if bytes.HasPrefix(line, []byte("g_page_config = ")) {
			jStr = line[16:]
			jStr = jStr[:len(jStr)-1]
			break
		}
	}
	if len(jStr) == 0 {
		return nil, fmt.Errorf("g_global_config not found")
	}
	return jStr, nil
}

type PageConfig struct {
	Mods map[string]struct {
		Status string
		Export bool
		Data   json.RawMessage
	}
}

func GetItems(data []byte) ([]Item, error) {
	var itemData struct {
		PostFeeText, Trace          string
		Auctions, RecommendAuctions []Item
		IsSameStyleView             bool
		Sellers                     []interface{}
		Query                       string
	}
	err := json.Unmarshal(data, &itemData)
	if err != nil {
		return nil, makeErr(err, "unmarshal")
	}
	return itemData.Auctions, nil
}
