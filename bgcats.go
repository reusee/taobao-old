package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func collectBackgroundCategories(backend Backend) {
	//clientSet := NewClientSet()
	//defer clientSet.Close()
	//clientSet.Logger = backend.LogClient

	client := http.DefaultClient

	cookie, err := ioutil.ReadFile("cookie")
	ce(err, "get cookie")

	var collect func(string, string)
	collect = func(path string, sid string) {
		if sid != "" {
			cat, err := strconv.Atoi(sid)
			ce(err, "strconv")
			info, err := backend.GetBgCatInfo(cat)
			ce(err, "get info")
			if time.Now().Sub(info.LastChecked) < time.Hour*24*5 {
				pt("skip %d\n", cat)
				return
			}
		}

		addr := "http://upload.taobao.com/auction/json/reload_cats.htm?customId="
		req, err := http.NewRequest("POST", addr, strings.NewReader(url.Values{
			"path": {path},
			"sid":  {sid},
		}.Encode()))
		ce(err, "new request")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", string(cookie))
		resp, err := client.Do(req)
		ce(err, "do")
		defer resp.Body.Close()
		content, err := ioutil.ReadAll(resp.Body)
		ce(err, "read body")
		r := transform.NewReader(bytes.NewReader(content), simplifiedchinese.GBK.NewDecoder())
		content, err = ioutil.ReadAll(r)
		ce(err, "conv gbk")

		var data []struct {
			Data []struct {
				Data []struct {
					Sid  string
					Name string
				}
			}
		}
		err = json.Unmarshal(content, &data)
		ce(err, "unmarshal")
		if len(data) == 0 {
			return
		}
		for _, c := range data[0].Data {
			for _, row := range c.Data {
				if strings.Contains(row.Sid, ":") { // pv
					continue
				}
				pt("%s %s\n", row.Sid, row.Name)
				parent := 0
				if sid != "" {
					parent, err = strconv.Atoi(sid)
					ce(err, "strconv")
				}
				cat, err := strconv.Atoi(row.Sid)
				ce(err, "strconv")
				backend.AddBgCat(Cat{
					Cat:    cat,
					Name:   row.Name,
					Parent: parent,
				})
				collect("next", row.Sid)
			}
		}

		cat, err := strconv.Atoi(sid)
		ce(err, "strconv")
		err = backend.SetBgCatInfo(cat, CatInfo{
			LastChecked: time.Now(),
		})
		ce(err, "set bgcat info")
		return
	}
	collect("all", "")
}
