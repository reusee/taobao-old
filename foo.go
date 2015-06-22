package main

import (
	"encoding/json"
	"net/http"
)

func foo() {
	clientSet := NewClientSet()
	defer clientSet.Close()

	cat := 50039716
	for page := 0; page < 20; page++ {
		pageUrl := sp("http://s.taobao.com/list?cat=%d&sort=sale-desc&bcoffset=0&s=%d", cat, page*60)
		pt("-> %s\n", pageUrl)
		clientSet.Do(func(client *http.Client) ClientState {
			bs, err := getBytes(client, pageUrl)
			if err != nil {
				//pt(sp("get %s error: %v\n", url, err))
				return Bad
			}
			jstr, err := GetPageConfigJson(bs)
			if err != nil {
				//pt(sp("get %s page config error: %v\n", url, err))
				return Bad
			}
			var config PageConfig
			err = json.Unmarshal(jstr, &config)
			if err != nil {
				//pt(sp("unmarshal %s json error: %v\n", url, err))
				return Bad
			}
			if config.Mods["itemlist"].Status == "hide" { // no items
				return Good
			}
			items, err := GetItems(config.Mods["itemlist"].Data)
			if err != nil {
				//pt(sp("unmarshal item list %s error: %v\n", url, err))
				return Bad
			}
			for _, item := range items {
				pt("%s\n", item.Title)
			}
			return Good
		})
		pt("\n")
	}
}
