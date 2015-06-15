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

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/reusee/catch"
	"github.com/reusee/hcutil"
)

var (
	ce = catch.PkgChecker("taobao")
	ct = catch.Catch
	pt = fmt.Printf
	sp = fmt.Sprintf
)

func main() {
	// database
	session, err := mgo.Dial("localhost")
	ce(err, "connect to db")
	defer session.Close()
	db := session.DB("taobao")
	_ = db

	allowDup := func(err error) error {
		if err, ok := err.(*mgo.LastError); ok {
			if err.Code == 11000 {
				return nil
			}
		}
		return err
	}

	// clients
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
	clients := make(chan *http.Client, len(proxies))
	for _, addr := range proxies {
		client, err := hcutil.NewClientSocks5("localhost:" + addr)
		ce(err, "new proxy "+addr)
		client.Timeout = time.Second * 8
		//pt("testing %s\n", addr)
		//_, err = hcutil.GetBytes(client, "http://www.taobao.com")
		//ce(err, "test client")
		//pt("proxy %s good\n", addr)
		clients <- client
	}

	now := time.Now()
	dateStr := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())

	jobsColle := db.C("jobs_" + dateStr)
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
	err = itemsColle.EnsureIndex(mgo.Index{
		Key:    []string{"nid"},
		Unique: true,
		Sparse: true,
	})
	ce(err, "ensure items collection index")

	markDone := func(cat, page int) {
		_, err = jobsColle.Find(bson.M{"cat": cat, "page": page}).Apply(mgo.Change{
			Update: bson.M{"done": true},
		}, nil)
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
		var err error
		go func() {
			defer ct(&err) // ignore errors
			defer func() {
				clients <- client
				wg.Done()
			}()
			url := sp("http://s.taobao.com/list?cat=%d&sort=sale-desc&bcoffset=0&s=%d", job.Cat, job.Page*60)
			bs, err := hcutil.GetBytes(client, url)
			ce(err, "get")
			jstr, err := GetPageConfigJson(bs)
			ce(err, "get page config")
			var config PageConfig
			err = json.Unmarshal(jstr, &config)
			ce(err, "unmarshal")
			items, err := GetItems(config.Mods["itemlist"].Data)
			ce(err, "get items")
			for _, item := range items {
				err = itemsColle.Insert(item)
				ce(err, "insert item")
			}
			pt("collected cat %d page %d, %d items\n", job.Cat, job.Page, len(items))
			if config.Mods["pager"].Status == "hide" || job.Page > 0 {
				markDone(job.Cat, job.Page)
				return
			}
			var pagerData struct {
				TotalPage int
			}
			err = json.Unmarshal(config.Mods["pager"].Data, &pagerData)
			ce(err, sp("get pager data: %s", config.Mods["pager"].Data))
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
		}()
	}
	wg.Wait()
	pt("collect %d page in %v\n", len(jobs), time.Now().Sub(t0))
	goto collect

}

type Item struct {
	//I2iTags       map[string]interface{}
	Nid           string
	Category      string
	Pid           string
	Title         string
	Raw_title     string
	Pic_url       string
	Detail_url    string
	View_price    string
	View_fee      string
	Item_loc      string
	Reserve_price string
	View_sales    string
	Comment_count string
	User_id       string
	Nick          string
	Shopcard      struct {
		LevelClasses []struct {
			LevelClass string
		}
		IsTmall         bool
		Delivery        []int
		Description     []int
		Service         []int
		EncryptedUserId string
		SellerCredit    int
		TotalRate       int
	}
	//Icon        interface{}
	Comment_url string
	ShopLink    string
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
