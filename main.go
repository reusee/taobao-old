package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
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
	//dumpUrl("http://s.taobao.com/search?q=LoveLive&commend=all&ssid=s5-e&search_type=item&sourceId=tb.index&spm=1.7274553.1997520841.1&initiative_id=tbindexz_20150607&bcoffset=-4&s=44")
	//pt("\n")
	//dumpUrl("http://s.taobao.com/search?q=LoveLive&commend=all&ssid=s5-e&search_type=item&sourceId=tb.index&spm=1.7274553.1997520841.1&initiative_id=tbindexz_20150607&bcoffset=-4&s=88")
	//pt("\n")
	//dumpUrl("http://s.taobao.com/search?q=LoveLive&commend=all&ssid=s5-e&search_type=item&sourceId=tb.index&spm=1.7274553.1997520841.1&initiative_id=tbindexz_20150607&bcoffset=-4&s=132")

	db, err := sqlx.Connect("pgx", "postgres://reus@localhost/taobao")
	ce(err, "connect db")
	defer db.Close()

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
		pt("testing %s\n", addr)
		client, err := hcutil.NewClientSocks5("localhost:" + addr)
		ce(err, "new proxy "+addr)
		client.Timeout = time.Second * 16
		_, err = hcutil.GetBytes(client, "http://www.taobao.com")
		ce(err, "test client")
		pt("proxy %s good\n", addr)
		clients <- client
	}

	/*
		for page := 0; page < 100; page++ {
			items, err := KeywordAndPage(client, "LoveLive", page)
			ce(err, sp("page %d", page))
			for _, item := range items {
				pt("%s\n", item.Raw_title)
			}
		}
	*/

	/*
		var collectCategory func(cat string)
		collectCategory = func(cat string) {
			bs, err := hcutil.GetBytes(client, sp("http://s.taobao.com/list?cat=%s", cat))
			ce(err, "get")
			jstr, err := GetPageConfigJson(string(bs))
			ce(err, "get page config")
			var config PageConfig
			err = json.Unmarshal(jstr, &config)
			ce(err, "unmarshal")
			var nav struct {
				Common []struct {
					Text string
					Sub  []struct {
						Text  string
						Key   string
						Value string
					}
				}
			}
			err = json.Unmarshal(config.Mods["nav"].Data, &nav)
			ce(err, "unmarshal")
			for _, e := range nav.Common {
				if e.Text == "相关分类" {
					for _, sub := range e.Sub {
						pt("%s %s\n", sub.Text, sub.Value)
						collectCategory(sub.Value)
					}
				}
			}
		}
		collectCategory("")
	*/

	urls := []string{}
	content, err := ioutil.ReadFile("categories")
	ce(err, "read categories file")
	t0 := time.Now()
	for _, line := range bytes.Split(content, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		catId := line[bytes.LastIndex(line, []byte(" "))+1:]
		urls = append(urls, sp("http://s.taobao.com/list?cat=%s&sort=sale-desc", catId))
	}
	var wg sync.WaitGroup
	wg.Add(len(urls))
	jobs := []string{}
	for _, url := range urls {
		client := <-clients
		url := url
		go func() {
			defer func() {
				wg.Done()
				clients <- client
			}()
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
			_, err = db.Exec("INSERT INTO raw (time, url, gob) VALUES ($1, $2, $3)",
				time.Now(), url, buf.Bytes())
			ce(err, "insert")
			pt("collected %s\n", url)
			if config.Mods["pager"].Status != "show" {
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
				jobs = append(jobs, sp("%s&bcoffset=0&s=%d", url, i*60))
			}
		}()
	}
	wg.Wait()
	pt("collect first page in %v, %d to go\n", time.Now().Sub(t0), len(jobs))

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

func KeywordAndPage(client *http.Client, keyword string, page int) ([]Item, error) {
	rawUrl := sp("http://s.taobao.com/search?q=%s&s=%d", keyword, 44*page)
	bs, err := hcutil.GetBytes(client, rawUrl)
	if err != nil {
		return nil, makeErr(err, sp("get %s", rawUrl))
	}
	jStr, err := GetPageConfigJson(bs)
	if err != nil {
		return nil, makeErr(err, "get g_page_config")
	}
	var config PageConfig
	err = json.Unmarshal(jStr, &config)
	if err != nil {
		return nil, makeErr(err, "decode")
	}
	return GetItems(config.Mods["itemlist"].Data)
}

func dumpUrl(rawUrl string) {
	u, err := url.Parse(rawUrl)
	ce(err, "parse url")
	query := u.Query()
	var keys []string
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		pt("%s -> %v\n", k, query[k])
	}
}

func dumpDoc(doc *goquery.Document) {
	html, _ := doc.Html()
	pt("%s\n", html)
}
