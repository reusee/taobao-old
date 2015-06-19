package main

import (
	"net/http"
	"time"

	"github.com/reusee/hcutil"
)

func ClientsProvider() (clientsIn, badClients chan<- *http.Client, clientsOut <-chan *http.Client, killClientsChan chan struct{}) {
	clientsIn, clientsOut, killClientsChan = NewClientsChan()
	bad := make(chan *http.Client)
	badClients = bad

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
				pt("client %s bad: %v\n", addr, err)
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
				pt("client %s ok\n", addr)
				clientsIn <- client
			}
		}
	}()

	// free proxies
	go provideFreeProxyClients(clientsIn)

	// reborn
	go func() {
		logs := make(map[*http.Client]int)
		for client := range bad {
			if logs[client] < 3 {
				clientsIn <- client
				logs[client]++
			}
		}
	}()

	return
}
