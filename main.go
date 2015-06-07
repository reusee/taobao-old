package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	client := &http.Client{
		Timeout: time.Second * 16,
	}

	rawUrl := "http://s.taobao.com/search?q=LoveLive"
	content, err := hcutil.GetBytes(client, rawUrl)
	ce(err, "get")

	res, err := parseResult(content)
	ce(err, "parse")
	_ = res

}

type Entry struct {
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

func parseResult(bs []byte) ([]Entry, error) {
	// get json string
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
	err := json.Unmarshal([]byte(jStr), &data)
	if err != nil {
		return nil, makeErr(err, "decode")
	}

	var itemData struct {
		PostFeeText, Trace          string
		Auctions, RecommendAuctions []Entry
		IsSameStyleView             bool
		Sellers                     []interface{} //TODO
		Query                       string
	}
	err = json.Unmarshal(data.Mods["itemlist"].Data, &itemData)
	if err != nil {
		return nil, makeErr(err, "unmarshal")
	}
	for _, item := range itemData.Auctions {
		pt("%v\n", item.Raw_title)
	}
	return nil, nil
}

func dumpUrl(rawUrl string) {
	u, err := url.Parse(rawUrl)
	ce(err, "parse url")
	query := u.Query()
	for k, v := range query {
		pt("%s -> %v\n", k, v)
	}
}

func dumpDoc(doc *goquery.Document) {
	html, _ := doc.Html()
	pt("%s\n", html)
}
