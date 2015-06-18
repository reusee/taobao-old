package main

import (
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/reusee/hcutil"
)

func provideFreeProxyClients(clients chan<- *http.Client) {
	entryPattern := regexp.MustCompile(`<tr><b><td>[0-9]+</td><td>([0-9.]+)</td><td>([0-9]+)</td>`)
	for page := 1; page <= 10; page++ {
		pageUrl := sp("http://www.proxy.com.ru/list_%d.html", page)
		bs, err := hcutil.GetBytes(http.DefaultClient, pageUrl)
		if err != nil {
			pt("error getting %s\n", pageUrl)
			continue
		}
		res := entryPattern.FindAllSubmatch(bs, -1)
		for _, match := range res {
			addr := sp("http://%s:%s", match[1], match[2])
			proxyUrl, err := url.Parse(addr)
			ce(err, sp("parse http proxy url %s", addr))
			client := &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(proxyUrl),
				},
				Timeout: time.Second * 8,
			}
			/*
				if testClient(client, addr) {
					clients <- client
				}
			*/
			clients <- client
		}
	}

	/*
		for page := 1; page <= 300; page++ {
			pageUrl := sp("http://www.kuaidaili.com/proxylist/%d/", page)
			bs, err := hcutil.GetBytes(http.DefaultClient, pageUrl)
			if err != nil {
				pt("error getting %s: %v\n", pageUrl, err)
				continue
			}
			doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bs))
			if err != nil {
				pt("error parsing %s: %v\n", pageUrl, err)
				continue
			}
			doc.Find("div#list table tbody tr").Each(func(i int, se *goquery.Selection) {
				tds := se.Find("td")
				addr := sp("%s:%s", tds.Get(0).FirstChild.Data, tds.Get(1).FirstChild.Data)
				proxyUrl, err := url.Parse(addr)
				ce(err, sp("parse http proxy url %s", addr))
				client := &http.Client{
					Transport: &http.Transport{
						Proxy: http.ProxyURL(proxyUrl),
					},
					Timeout: time.Second * 8,
				}
				if testClient(client, addr) {
					clients <- client
				}
			})
		}
	*/
}

func testClient(client *http.Client, addr string) bool {
	pt("testing proxy %s\n", addr)
	done := make(chan struct{})
	go func() {
		_, err := hcutil.GetBytes(client, "http://www.taobao.com")
		if err == nil {
			close(done)
		} else {
			pt("client get error %s: %v\n", addr, err)
		}
	}()
	select {
	case <-time.After(time.Second * 8):
		pt("client test timeout %s\n", addr)
	case <-done:
		pt("client %s ok\n", addr)
		return true
	}
	pt("client %s not ok\n", addr)
	return false
}
