package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func collect(backend Backend) {
	// client set
	clientSet := NewClientSet()
	defer clientSet.Close()
	clientSet.Logger = backend.LogClient

	// first-page jobs
	jobs := []Job{}
	cats, err := backend.GetCats()
	ce(err, "get cats")
	for _, cat := range cats {
		jobs = append(jobs, Job{
			Cat:  cat.Cat,
			Page: 0,
			Done: false,
		})
	}
	ce(backend.AddJobs(jobs), "add jobs")

	markDone := func(job Job) {
		err := backend.DoneJob(job)
		ce(err, "mark done")
	}

	// status
	var itemsCount uint64
	var jobsTotal int64
	var jobsDone int64
	go func() {
		for range time.NewTicker(time.Second * 3).C {
			pt("%d / %d jobs done. %d items collected\n",
				atomic.LoadInt64(&jobsDone),
				jobsTotal,
				atomic.LoadUint64(&itemsCount))
		}
	}()

	// collect
collect:
	jobs, err = backend.GetJobs()
	ce(err, "get jobs")
	pt("%d jobs\n", len(jobs))
	if len(jobs) == 0 {
		return
	}
	var wg sync.WaitGroup
	wg.Add(len(jobs))
	jobsTotal = int64(len(jobs))
	jobsDone = 0
	sem := make(chan struct{}, 256)
	for _, job := range jobs {
		job := job
		sem <- struct{}{}
		go func() {
			defer func() {
				wg.Done()
				atomic.AddInt64(&jobsDone, 1)
				<-sem
			}()
			url := sp("http://s.taobao.com/list?cat=%d&sort=sale-desc&bcoffset=0&s=%d", job.Cat, job.Page*60)
			clientSet.Do(func(client *http.Client) ClientState {
				bs, err := getBytes(client, url)
				if err != nil {
					//pt(sp("get %s error: %v\n", url, err))
					return Bad
				}
				jstr, err := GetPageConfigJson(bs)
				if err != nil {
					//pt(sp("get %s page config error: %v\n", url, err))
					return Bad
				}
				var config PageConfig
				err = json.Unmarshal(jstr, &config)
				if err != nil {
					//pt(sp("unmarshal %s json error: %v\n", url, err))
					return Bad
				}
				if config.Mods["itemlist"].Status == "hide" { // no items
					markDone(job)
					return Good
				}
				items, err := GetItems(config.Mods["itemlist"].Data)
				if err != nil {
					//pt(sp("unmarshal item list %s error: %v\n", url, err))
					return Bad
				}
				for {
					if backend.AddItems(items, job) == nil {
						break
					}
				}
				atomic.AddUint64(&itemsCount, uint64(len(items)))
				if config.Mods["pager"].Status == "hide" || job.Page > 0 {
					markDone(job)
					return Good
				}
				var pagerData struct {
					TotalPage int
				}
				err = json.Unmarshal(config.Mods["pager"].Data, &pagerData)
				if err != nil {
					//pt(sp("unmarshal pager %s error: %v\n", url, err))
					return Bad
				}
				maxPage := 20
				if pagerData.TotalPage < maxPage {
					maxPage = pagerData.TotalPage
				}
				js := []Job{}
				for i := 1; i < maxPage; i++ {
					js = append(js, Job{
						Cat:  job.Cat,
						Page: i,
						Done: false,
					})
				}
				ce(backend.AddJobs(js), "add jobs")
				markDone(job)
				return Good
			})
		}()
	}
	wg.Wait()
	goto collect

}

func GetPageConfigJson(content []byte) ([]byte, error) {
	var jStr []byte
	for _, line := range bytes.Split(content, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		line = bytes.TrimLeft(line, " ")
		if bytes.HasPrefix(line, []byte("g_page_config = ")) {
			jStr = line[16:]
			jStr = jStr[:len(jStr)-1]
			break
		}
	}
	if len(jStr) == 0 {
		return nil, fmt.Errorf("g_global_config not found")
	}
	return jStr, nil
}

type PageConfig struct {
	Mods map[string]struct {
		Status string
		Export bool
		Data   json.RawMessage
	}
}

func GetItems(data []byte) ([]Item, error) {
	var itemData struct {
		PostFeeText, Trace          string
		Auctions, RecommendAuctions []Item
		IsSameStyleView             bool
		Sellers                     []interface{}
		Query                       string
	}
	err := json.Unmarshal(data, &itemData)
	if err != nil {
		return nil, makeErr(err, "unmarshal")
	}
	return itemData.Auctions, nil
}
