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
	Subs []Cat
}

func collectCategories(db *mgo.Database, client *http.Client) {
	root := collectCategory(client, "")

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

func collectCategory(client *http.Client, cat string) (ret Cat) {
	if cat != "" {
		id, err := strconv.Atoi(cat)
		ce(err, sp("parse cat id %s", cat))
		ret.Cat = id
	}
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
				sub := collectCategory(client, sub.Value)
				ret.Subs = append(ret.Subs, sub)
			}
		}
	}
	return
}
