package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/reusee/hcutil"
)

type Cat struct {
	Cat  int
	Name string
	Subs []Cat
}

func collectCategories(db *mgo.Database) {
	collected := make(map[string]bool)
	var collectCategory func(cat, name string) Cat
	collectCategory = func(cat, name string) (ret Cat) {
		if collected[cat] {
			panic(sp("loop %s %s", cat, name))
		}
		client := http.DefaultClient
		if cat != "" {
			id, err := strconv.Atoi(cat)
			ce(err, sp("parse cat id %s", cat))
			ret.Cat = id
			ret.Name = name
		}
		for {
			bs, err := hcutil.GetBytes(client, sp("http://s.taobao.com/list?cat=%s", cat))
			if err != nil {
				continue
			}
			jstr, err := GetPageConfigJson(bs)
			if err != nil {
				continue
			}
			var config PageConfig
			err = json.Unmarshal(jstr, &config)
			if err != nil {
				continue
			}
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
			if err != nil {
				continue
			}
			for _, e := range nav.Common {
				if e.Text == "相关分类" {
					for _, sub := range e.Sub {
						pt("%s %s\n", sub.Text, sub.Value)
						ret.Subs = append(ret.Subs, collectCategory(sub.Value, sub.Text))
					}
				}
			}
			break
		}
		collected[cat] = true
		return
	}

	root := collectCategory("", "")

	catsColle := db.C("cats")
	err := catsColle.EnsureIndex(mgo.Index{
		Key:    []string{"cat"},
		Unique: true,
	})
	ce(err, "ensure index")
	for _, cat := range root.Subs {
		_, err = catsColle.Upsert(bson.M{"cat": cat.Cat}, cat)
		ce(err, "upsert")
	}
}
