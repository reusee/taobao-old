package taobao

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

func (m *Mysql) Foo() {
	switch os.Args[2] {
	case "collect":
		clientSet := NewClientSet()
		defer clientSet.Close()
		sem := make(chan struct{}, 32)
		for {
			sem <- struct{}{}
			go func() {
				defer func() {
					<-sem
				}()
				pageUrl := "http://s.taobao.com/list?cat=50001739&bcoffset=0&s=120"
				clientSet.Do(func(client *http.Client) ClientState {
					bs, err := getBytes(client, pageUrl)
					if err != nil {
						pt("get error\n")
						return Bad
					}
					jstr, err := GetPageConfigJson(bs)
					if err != nil {
						pt("get page config error\n")
						return Bad
					}
					var config PageConfig
					err = json.Unmarshal(jstr, &config)
					if err != nil {
						pt("unmarshal error\n")
						return Bad
					}
					_, err = m.db.Exec(`INSERT INTO foo (data) VALUES (?)`, jstr)
					ce(err, "insert")
					//if config.Mods["itemlist"].Status == "hide" { // no items
					//	return Good
					//}
					//items, err := GetItems(config.Mods["itemlist"].Data)
					//if err != nil {
					//	//pt(sp("unmarshal item list %s error: %v\n", url, err))
					//	return Bad
					//}
					//for _, item := range items {
					//	pt("%s\n", item.Title)
					//}
					return Good
				})
			}()
		}

	case "check":
		rows, err := m.db.Query(`SELECT data FROM foo ORDER BY id ASC`)
		ce(err, "query")
		for rows.Next() {
			var bs []byte
			ce(rows.Scan(&bs), "scan")
			var config PageConfig
			if err := json.Unmarshal(bs, &config); err != nil {
				pt(sp("unmarshal error %v\n", err))
				ioutil.WriteFile("j", bs, 0644)
				continue
			}
			items, err := GetItems(config.Mods["itemlist"].Data)
			ce(err, "get items")
			for _, item := range items {
				if !strings.Contains(item.Title, "狗") {
					pt("%s\n", item.Title)
				}
			}
			//		var next string
			//		fmt.Scanf("%s\n", &next)
			//		switch next {
			//		case "1":
			//			buf := new(bytes.Buffer)
			//			json.Indent(buf, bs, "", "\t")
			//			ce(ioutil.WriteFile("1", buf.Bytes(), 0644), "write file")
			//		case "2":
			//			buf := new(bytes.Buffer)
			//			json.Indent(buf, bs, "", "\t")
			//			ce(ioutil.WriteFile("2", buf.Bytes(), 0644), "write file")
			//		}
		}
		ce(rows.Err(), "rows")

	case "find":
		cat := 50071817
		date := "20150623"
		rows, err := m.db.Query(sp(`SELECT data, page FROM jobs_%s WHERE cat = ? ORDER BY page ASC`, date), cat)
		ce(err, "query")
		for rows.Next() {
			var jsn []byte
			var page int
			ce(rows.Scan(&jsn, &page), "scan")
			var config PageConfig
			ce(json.Unmarshal(jsn, &config), "unmarshal")
			items, err := GetItems(config.Mods["itemlist"].Data)
			ce(err, "get items")
			pt("%d\n", page)
			for _, item := range items {
				pt("%s\n", item.Title)
			}
			var next string
			fmt.Scanf("%s", &next)
			if len(next) > 0 {
				buf := new(bytes.Buffer)
				ce(json.Indent(buf, jsn, "", "\t"), "indent")
				ioutil.WriteFile(next, buf.Bytes(), 0644)
			}
		}
		ce(rows.Err(), "rows")
	}
}
