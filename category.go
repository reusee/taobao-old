package main

import (
	"encoding/json"
	"strconv"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/reusee/hcutil"
)

type Cat struct {
	Cat  int
	Subs []Cat
}

func collectCategories(db *mgo.Database) {
	clientsIn, _, clientsOut, killClientsChan := ClientsProvider()
	defer close(killClientsChan)

	var collectCategory func(cat string) Cat
	collectCategory = func(cat string) (ret Cat) {
		if cat != "" {
			id, err := strconv.Atoi(cat)
			ce(err, sp("parse cat id %s", cat))
			ret.Cat = id
		}
		for {
			client := <-clientsOut
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
			clientsIn <- client
			subs := []string{}
			for _, e := range nav.Common {
				if e.Text == "相关分类" {
					for _, sub := range e.Sub {
						pt("%s %s\n", sub.Text, sub.Value)
						subs = append(subs, sub.Value)
					}
				}
			}
			retsChan := make(chan Cat)
			for _, sub := range subs {
				sub := sub
				go func() {
					retsChan <- collectCategory(sub)
				}()
			}
			for i := 0; i < len(subs); i++ {
				ret.Subs = append(ret.Subs, <-retsChan)
			}
			break
		}
		return
	}

	root := collectCategory("")

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
