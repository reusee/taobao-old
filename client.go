package main

import (
	"net/http"
	"time"

	"github.com/reusee/hcutil"
)

type ClientSet struct {
	in   chan<- *http.Client
	out  <-chan *http.Client
	bad  chan<- *http.Client
	kill chan struct{}
}

func NewClientSet() *ClientSet {
	in, out, kill := NewClientsChan()
	bad := make(chan *http.Client)

	// local ss proxies
	go func() {
		proxies := []string{
			"8022",
			"8032",
			"8080",
			"8010",
			"8031",
			"8030",
			//"8023",
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
			done := make(chan struct{})
			go func() {
				_, err = hcutil.GetBytes(client, "http://www.taobao.com")
				if err == nil {
					close(done)
				}
			}()
			select {
			case <-time.After(time.Second * 4):
			case <-done:
				in <- client
			}
		}
	}()

	// free proxies
	go provideFreeProxyClients(in)

	// reborn
	go func() {
		logs := make(map[*http.Client]int)
		for client := range bad {
			if logs[client] < 3 {
				in <- client
				logs[client]++
			}
		}
	}()

	return &ClientSet{
		in:   in,
		out:  out,
		bad:  bad,
		kill: kill,
	}
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
		switch fn(client) {
		case Good:
			s.in <- client
			break loop
		case Bad:
			s.bad <- client
		}
	}
}

func (s *ClientSet) Close() {
	close(s.kill)
}
