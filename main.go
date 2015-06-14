package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"

	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/reusee/catch"
	"github.com/reusee/hcutil"
)

var (
	ce = catch.PkgChecker("taobao")
	pt = fmt.Printf
	sp = fmt.Sprintf
)

func main() {
	// database
	db, err := sqlx.Connect("pgx", "postgres://reus@localhost/taobao")
	ce(err, "connect db")
	defer db.Close()

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
		client.Timeout = time.Second * 16
		//pt("testing %s\n", addr)
		//_, err = hcutil.GetBytes(client, "http://www.taobao.com")
		//ce(err, "test client")
		//pt("proxy %s good\n", addr)
		clients <- client
	}

	now := time.Now()
	dateStr := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())

	// first-page jobs
	jobsTableName := "jobs_" + dateStr
	_, err = db.Exec(sp(`CREATE TABLE IF NOT EXISTS %s (
		cat BIGINT,
		page INTEGER,
		done BOOL NOT NULL DEFAULT FALSE,
		PRIMARY KEY (cat, page))`, jobsTableName))
	ce(err, "create jobs table")
	content, err := ioutil.ReadFile("categories")
	ce(err, "read categories file")
	for _, line := range bytes.Split(content, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		catStr := line[bytes.LastIndex(line, []byte(" "))+1:]
		cat, err := strconv.Atoi(string(catStr))
		ce(err, "parse cat id")
		_, err = db.Exec(sp(`INSERT INTO %s (cat, page) SELECT $1, $2
			WHERE NOT EXISTS (SELECT 1 FROM %s WHERE cat = $1 AND page = $2)`,
			jobsTableName, jobsTableName), cat, 0)
		ce(err, "insert first-page job")
	}
	pt("first-page jobs inserted\n")

	// raw table
	rawTableName := "raws_" + dateStr
	_, err = db.Exec(sp(`CREATE TABLE IF NOT EXISTS %s (
		cat BIGINT,
		page INTEGER,
		gob BYTEA,
		PRIMARY KEY (cat, page))`, rawTableName))
	ce(err, "create raws table")

	// collect
collect:
	jobs := []struct {
		Cat  uint64
		Page int
	}{}
	err = db.Select(&jobs, sp(`SELECT cat,page FROM %s WHERE done = FALSE`, jobsTableName))
	ce(err, "get jobs")
	if len(jobs) == 0 {
		return
	}
	pt("%d jobs\n", len(jobs))
	var wg sync.WaitGroup
	wg.Add(len(jobs))
	t0 := time.Now()
	for _, job := range jobs {
		client := <-clients
		job := job
		go func() {
			defer func() {
				wg.Done()
				clients <- client
				_, err = db.Exec(sp("UPDATE %s SET DONE = TRUE WHERE cat = $1 AND page = $2", jobsTableName),
					job.Cat, job.Page)
				ce(err, "update job table")
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
			buf := new(bytes.Buffer)
			err = gob.NewEncoder(buf).Encode(items)
			ce(err, "encode gob")
			_, err = db.Exec(sp(`INSERT INTO %s (cat, page, gob) SELECT $1, $2, $3
				WHERE NOT EXISTS (SELECT 1 FROM %s WHERE cat = $1 AND page = $2)`, rawTableName, rawTableName),
				job.Cat, job.Page, buf.Bytes())
			ce(err, "insert")
			pt("collected cat %d page %d, %d items\n", job.Cat, job.Page, len(items))
			if config.Mods["pager"].Status == "hide" || job.Page > 0 {
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
				_, err = db.Exec(sp(`INSERT INTO %s (cat, page) SELECT $1, $2
					WHERE NOT EXISTS (SELECT 1 FROM %s WHERE cat = $1 AND page = $2)`, jobsTableName, jobsTableName),
					job.Cat, i)
				ce(err, "insert job")
			}
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
	//Icon        interface{} // TODO
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
		Sellers                     []interface{} //TODO
		Query                       string
	}
	err := json.Unmarshal(data, &itemData)
	if err != nil {
		return nil, makeErr(err, "unmarshal")
	}
	return itemData.Auctions, nil
}
