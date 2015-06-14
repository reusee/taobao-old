package main

import (
	"encoding/json"
	"net/http"

	"github.com/reusee/hcutil"
)

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
