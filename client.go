package taobao

import (
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/reusee/hcutil"
)

type ClientSet struct {
	sync.RWMutex
	in     chan<- *http.Client
	out    <-chan *http.Client
	kill   chan struct{}
	good   map[*http.Client]int
	Logger func(ClientInfo, ClientState)
	infos  map[*http.Client]ClientInfo
}

type ClientInfo struct {
	HttpProxyAddr string
}

func NewClientSet() *ClientSet {
	in, out, kill := NewClientsChan()

	s := &ClientSet{
		in:    in,
		out:   out,
		kill:  kill,
		good:  make(map[*http.Client]int),
		infos: make(map[*http.Client]ClientInfo),
	}

	// local ss proxies
	go func() {
		proxies := []string{
			"8022",
			"8032",
			"8080",
			"8010",
			"8031",
			"8030",
			"8023",
			"8020",
			"8021",
			"8040",
			"8015",
		}
		for _, addr := range proxies {
			client, err := hcutil.NewClientSocks5("localhost:" + addr)
			if err != nil {
				continue
			}
			client.Timeout = time.Second * 32
			in <- client
		}
	}()

	// free proxies
	go s.provideFreeProxyClients()

	return s
}

type ClientState uint8

const (
	Good ClientState = iota
	Bad
)

func (s *ClientSet) Do(fn func(client *http.Client) ClientState) {
loop:
	for {
		client := <-s.out
		s.Lock()
		if _, ok := s.good[client]; !ok {
			s.good[client] = 32
		}
		s.Unlock()
		switch fn(client) {
		case Good:
			s.in <- client
			s.Lock()
			s.good[client]++
			if s.Logger != nil {
				s.Logger(s.infos[client], Good)
			}
			s.Unlock()
			break loop
		case Bad:
			s.Lock()
			if s.good[client] > 0 {
				s.good[client]--
				s.in <- client
			}
			if s.Logger != nil {
				s.Logger(s.infos[client], Bad)
			}
			s.Unlock()
		}
	}
}

func (s *ClientSet) Close() {
	close(s.kill)
}

func (s *ClientSet) provideFreeProxyClients() {
	entryPattern := regexp.MustCompile(`<tr><b><td>[0-9]+</td><td>([0-9.]+)</td><td>([0-9]+)</td>`)
	for page := 1; page <= 10; page++ {
		pageUrl := sp("http://www.proxy.com.ru/list_%d.html", page)
		var bs []byte
		var err error
		s.Do(func(client *http.Client) ClientState {
			bs, err = getBytes(client, pageUrl)
			if err != nil {
				return Bad
			}
			return Good
		})
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
				Timeout: time.Second * 32,
			}
			s.Lock()
			s.infos[client] = ClientInfo{
				HttpProxyAddr: addr,
			}
			s.Unlock()
			time.Sleep(time.Millisecond * 200)
			s.in <- client
		}
	}

	/*
		for page := 1; page <= 300; page++ {
			pageUrl := sp("http://www.kuaidaili.com/proxylist/%d/", page)
			bs, err := getBytes(http.DefaultClient, pageUrl)
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
					Timeout: time.Second * 32,
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
		_, err := getBytes(client, "http://www.taobao.com")
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
