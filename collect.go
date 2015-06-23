package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var MaxPage = 20

func collect(backend Backend) {
	if len(os.Args) > 2 {
		page, err := strconv.Atoi(os.Args[2])
		ce(err, "parse max page")
		MaxPage = page
	}

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
					return Bad
				}
				jstr, err := GetPageConfigJson(bs)
				if err != nil {
					return Bad
				}
				job.Data = jstr
				var config PageConfig
				if json.Unmarshal(jstr, &config) != nil {
					return Bad
				}
				// check category in maininfo
				catId, err := strconv.Atoi(config.MainInfo.SrpGlobal.Cat)
				ce(err, "parse cat id in main info")
				if catId != job.Cat {
					return Bad
				}
				// check category in mod nav data
				var navData NavData
				if json.Unmarshal(config.Mods["nav"].Data, &navData) != nil {
					return Bad
				}
				if navData.Hidenav { // items not belong to this category
					return Bad
				}
				catPath := navData.Breadcrumbs.Catpath
				lastCatidStr := catPath[len(catPath)-1].Catid
				lastCatid, err := strconv.Atoi(lastCatidStr)
				ce(err, sp("parse cat id %s", lastCatidStr))
				if lastCatid != job.Cat {
					return Bad
				}
				// get items
				if config.Mods["itemlist"].Status == "hide" { // no items
					markDone(job)
					return Good
				}
				items, err := GetItems(config.Mods["itemlist"].Data)
				if err != nil {
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
				if json.Unmarshal(config.Mods["pager"].Data, &pagerData) != nil {
					return Bad
				}
				maxPage := MaxPage
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
	MainInfo struct {
		SrpGlobal struct {
			Cat string
		}
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
