package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
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

	for page := 0; page < 100; page++ {
		items, err := KeywordAndPage(client, "LoveLive", page)
		ce(err, sp("page %d", page))
		for _, item := range items {
			pt("%s\n", item.Raw_title)
		}
	}

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

func KeywordAndPage(client *http.Client, keyword string, page int) ([]Item, error) {
	rawUrl := sp("http://s.taobao.com/search?q=%s&s=%d", keyword, 44*page)
	bs, err := hcutil.GetBytes(client, rawUrl)
	if err != nil {
		return nil, makeErr(err, sp("get %s", rawUrl))
	}
	content := string(bs)
	var jStr string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimLeft(line, " ")
		if strings.HasPrefix(line, "g_page_config = ") {
			jStr = line[16:]
			jStr = jStr[:len(jStr)-1]
			break
		}
	}
	if len(jStr) == 0 {
		return nil, fmt.Errorf("no data")
	}
	var data struct {
		Mods map[string]struct {
			Status string
			Export bool
			Data   json.RawMessage
		}
	}
	err = json.Unmarshal([]byte(jStr), &data)
	if err != nil {
		return nil, makeErr(err, "decode")
	}

	var itemData struct {
		PostFeeText, Trace          string
		Auctions, RecommendAuctions []Item
		IsSameStyleView             bool
		Sellers                     []interface{} //TODO
		Query                       string
	}
	err = json.Unmarshal(data.Mods["itemlist"].Data, &itemData)
	if err != nil {
		return nil, makeErr(err, "unmarshal")
	}
	if page == 0 {
		if len(itemData.Auctions) != 48 {
			return nil, fmt.Errorf("wrong result count, got %d", len(itemData.Auctions))
		}
	} else {
		if len(itemData.Auctions) != 44 {
			return nil, fmt.Errorf("wrong result count, got %d", len(itemData.Auctions))
		}
	}
	return itemData.Auctions, nil
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
