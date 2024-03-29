package taobao

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
)

func CollectForegroundCategories(backend Backend) {
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
			bs, err := getBytes(client, sp("http://s.taobao.com/list?cat=%s", catStr))
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
			var nav NavData
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
			ce(backend.AddFgCat(cat), "add cat")
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
