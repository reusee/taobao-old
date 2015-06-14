package main

import (
	"encoding/json"
	"net/http"

	"github.com/reusee/hcutil"
)

func collectCategories(client *http.Client) {
	var collectCategory func(cat string)
	collectCategory = func(cat string) {
		bs, err := hcutil.GetBytes(client, sp("http://s.taobao.com/list?cat=%s", cat))
		ce(err, "get")
		jstr, err := GetPageConfigJson(bs)
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
}
