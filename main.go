package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/PuerkitoBio/goquery"
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

	client := &http.Client{
		Timeout: time.Second * 16,
	}
	_ = client

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

	content, err := ioutil.ReadFile("categories")
	ce(err, "read categories file")
	jobs := 0
	t0 := time.Now()
	for _, line := range bytes.Split(content, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		catId := line[bytes.LastIndex(line, []byte(" "))+1:]
		// collect first page
		url := sp("http://s.taobao.com/list?cat=%s&sort=sale-desc", catId)
		bs, err := hcutil.GetBytes(client, url)
		ce(err, "get")
		jstr, err := GetPageConfigJson(bs)
		ce(err, "get page config")
		var config PageConfig
		err = json.Unmarshal(jstr, &config)
		ce(err, "unmarshal")
		items, err := GetItems(config.Mods["itemlist"].Data)
		ce(err, "get items")
		pt("%s %d\n", catId, len(items))
		// get page count
		if config.Mods["pager"].Status != "show" {
			continue
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
		jobs += maxPage
	}
	pt("collect first page in %v, %d to go\n", time.Now().Sub(t0), jobs)

}

type Item struct {
	I2iTags       map[string]interface{}
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
	Icon        interface{} // TODO
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
