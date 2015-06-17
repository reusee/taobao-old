package main

import (
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/reusee/hcutil"
)

func provideFreeProxyClients(clients chan *http.Client) {
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
			done := make(chan struct{})
			go func() {
				_, err = hcutil.GetBytes(client, "http://www.taobao.com")
				if err == nil {
					close(done)
				}
			}()
			select {
			case <-time.After(time.Second * 3):
			case <-done:
				pt("client %s ok\n", addr)
				clients <- client
			}
		}
	}
}
