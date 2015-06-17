package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/reusee/hcutil"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func collect(db *mgo.Database) {
	// client provider
	clients := make(chan *http.Client, 1024)
	go func() {
		proxies := []string{
			"8022",
			"8032",
			"8080",
			"8010",
			"8031",
			"8030",
			//"8023",
			"8020",
			"8021",
			"8040",
			"8015",
		}
		for _, addr := range proxies {
			client, err := hcutil.NewClientSocks5("localhost:" + addr)
			if err != nil {
				pt("client %s bad: %v\n", addr, err)
				continue
			}
			client.Timeout = time.Second * 8
			done := make(chan struct{})
			go func() {
				_, err = hcutil.GetBytes(client, "http://www.taobao.com")
				if err == nil {
					close(done)
				}
			}()
			select {
			case <-time.After(time.Second * 3):
			case <-done:
				pt("client %s ok\n", addr)
				clients <- client
			}
		}
	}()
	go provideFreeProxyClients(clients)

	now := time.Now()
	dateStr := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())

	jobsColle := db.C("jobs_" + dateStr)
	err := jobsColle.Create(&mgo.CollectionInfo{
		Extra: bson.M{
			"compression": "zlib",
		},
	})
	ce(ignoreExistsColle(err), "create jobs collection")
	err = jobsColle.EnsureIndex(mgo.Index{
		Key:    []string{"cat", "page"},
		Unique: true,
		Sparse: true,
	})
	ce(err, "ensure jobs collection index")
	err = jobsColle.EnsureIndexKey("done")
	ce(err, "ensure jobs collection done key")
	err = jobsColle.EnsureIndexKey("cat")
	ce(err, "ensure jobs collection cat key")
	err = jobsColle.EnsureIndexKey("page")
	ce(err, "ensure jobs collection page key")

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
		err = jobsColle.Insert(Job{
			Cat:  cat,
			Page: 0,
			Done: false,
		})
		ce(allowDup(err), "insert job")
	}
	pt("first-page jobs inserted\n")

	itemsColle := db.C("items_" + dateStr)
	err = itemsColle.Create(&mgo.CollectionInfo{
		Extra: bson.M{
			"compression": "zlib",
		},
	})
	ce(ignoreExistsColle(err), "create items collection")
	err = itemsColle.EnsureIndex(mgo.Index{
		Key:    []string{"nid"},
		Unique: true,
		Sparse: true,
	})
	ce(err, "ensure items collection index")

	markDone := func(cat, page int) {
		err := jobsColle.Update(bson.M{"cat": cat, "page": page},
			bson.M{"$set": bson.M{"done": true}})
		ce(err, "mark done")
	}

	// collect
collect:
	jobs := []Job{}
	err = jobsColle.Find(bson.M{"done": false}).All(&jobs)
	ce(err, "get jobs")
	pt("%d jobs\n", len(jobs))
	if len(jobs) == 0 {
		return
	}
	var wg sync.WaitGroup
	wg.Add(len(jobs))
	t0 := time.Now()
	for _, job := range jobs {
		client := <-clients
		job := job
		go func() {
			defer func() {
				wg.Done()
			}()
			t0 := time.Now()
			url := sp("http://s.taobao.com/list?cat=%d&sort=sale-desc&bcoffset=0&s=%d", job.Cat, job.Page*60)
			bs, err := hcutil.GetBytes(client, url)
			if err != nil {
				pt(sp("get %s error: %v", url, err))
				return
			}
			jstr, err := GetPageConfigJson(bs)
			if err != nil {
				pt(sp("get %s page config error: %v", url, err))
				return
			}
			var config PageConfig
			err = json.Unmarshal(jstr, &config)
			if err != nil {
				pt(sp("unmarshal %s json error: %v", url, err))
				return
			}
			if config.Mods["itemlist"].Status == "hide" { // no items
				markDone(job.Cat, job.Page)
				clients <- client
				return
			}
			items, err := GetItems(config.Mods["itemlist"].Data)
			if err != nil {
				pt(sp("unmarshal item list %s error: %v", url, err))
				return
			}
			for _, item := range items {
				err = itemsColle.Insert(item)
				ce(allowDup(err), "insert item")
			}
			pt("collected cat %d page %d, %d items, %v\n", job.Cat, job.Page, len(items), time.Now().Sub(t0))
			if config.Mods["pager"].Status == "hide" || job.Page > 0 {
				markDone(job.Cat, job.Page)
				clients <- client
				return
			}
			var pagerData struct {
				TotalPage int
			}
			err = json.Unmarshal(config.Mods["pager"].Data, &pagerData)
			if err != nil {
				pt(sp("unmarshal pager %s error: %v", url, err))
				return
			}
			maxPage := 10
			if pagerData.TotalPage < maxPage {
				maxPage = pagerData.TotalPage
			}
			for i := 1; i < maxPage; i++ {
				err = jobsColle.Insert(Job{
					Cat:  job.Cat,
					Page: i,
					Done: false,
				})
				ce(allowDup(err), "insert job")
			}
			markDone(job.Cat, job.Page)
			clients <- client
		}()
	}
	wg.Wait()
	pt("collect %d page in %v\n", len(jobs), time.Now().Sub(t0))
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
