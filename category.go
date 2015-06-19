package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/reusee/hcutil"
)

type Cat struct {
	Cat       int
	Name      string
	Relatives []int
}

func collectCategories(db *mgo.Database) {
	catsColle := db.C("cats")
	err := catsColle.EnsureIndex(mgo.Index{
		Key:    []string{"cat"},
		Unique: true,
	})
	ce(err, "ensure index")

	cats := make(map[int]Cat)
	clientSet := NewClientSet()
	var collectCategory func(Cat)
	collectCategory = func(cat Cat) {
		if _, ok := cats[cat.Cat]; ok {
			return // skip
		}
		pt("%10d %s\n", cat.Cat, cat.Name)
		var relatives []Cat
		clientSet.Do(func(client *http.Client) ClientState {
			var catStr string
			if cat.Cat != 0 {
				catStr = strconv.Itoa(cat.Cat)
			}
			bs, err := hcutil.GetBytes(client, sp("http://s.taobao.com/list?cat=%s", catStr))
			if err != nil {
				return Bad
			}
			jstr, err := GetPageConfigJson(bs)
			if err != nil {
				return Bad
			}
			var config PageConfig
			err = json.Unmarshal(jstr, &config)
			if err != nil {
				return Bad
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
				return Bad
			}
			for _, e := range nav.Common {
				if e.Text == "相关分类" {
					for _, sub := range e.Sub {
						id, err := strconv.Atoi(sub.Value)
						ce(err, sp("parse cat id %s", sub.Value))
						relatives = append(relatives, Cat{
							Cat:  id,
							Name: sub.Text,
						})
						cat.Relatives = append(cat.Relatives, id)
					}
				}
			}
			return Good
		})
		if cat.Cat != 0 {
			cats[cat.Cat] = cat
			_, err = catsColle.Upsert(bson.M{"cat": cat.Cat}, cat)
			ce(err, "upsert")
		}

		wg := new(sync.WaitGroup)
		wg.Add(len(relatives))
		for _, r := range relatives {
			r := r
			go func() {
				defer wg.Done()
				collectCategory(r)
			}()
		}
		wg.Wait()
	}

	collectCategory(Cat{})

}
