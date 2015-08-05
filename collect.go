package taobao

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var jobTraceSet = NewTraceSet()

func Collect(backend Backend) {
	// client set
	clientSet := NewClientSet()
	defer clientSet.Close()
	clientSet.Logger = backend.LogClient

	jobsIn, jobsOut, jobsClose := NewJobsChan()
	defer close(jobsClose)
	wg := new(sync.WaitGroup)

	fgcats, err := backend.GetFgCats()
	ce(err, "get fgcats")
	wg.Add(len(fgcats))
	go func() {
		for _, cat := range fgcats {
			jobsIn <- Job{
				Cat:  cat.Cat,
				Page: 0,
			}
		}
	}()

	var jobsDone, itemsCollected, totalJobsDone uint64
	go func() {
		ticker := time.NewTicker(time.Second * 10)
		t0 := time.Now()
		for range ticker.C {
			pt("%d / %d / %d - %v\n", atomic.SwapUint64(&jobsDone, 0),
				atomic.SwapUint64(&itemsCollected, 0),
				atomic.LoadUint64(&totalJobsDone), time.Now().Sub(t0))
		}
	}()

	go func() {
		sem := make(chan struct{}, 128)
		for {
			job := <-jobsOut
			sem <- struct{}{}
			go func() {
				defer func() {
					wg.Done()
					atomic.AddUint64(&jobsDone, 1)
					atomic.AddUint64(&totalJobsDone, 1)
					<-sem
				}()
				// check
				if job.Page > 99 {
					return
				}
				if backend.IsCollected(job) {
					wg.Add(1)
					jobsIn <- Job{
						Cat:  job.Cat,
						Page: job.Page + 1,
					}
					return
				}
				// trace
				tc := jobTraceSet.NewTrace(sp("cat %d, page %d", job.Cat, job.Page))
				defer tc.SetFlag("done")
				// collect
				url := sp("http://s.taobao.com/list?cat=%d&sort=sale-desc&bcoffset=0&s=%d", job.Cat, job.Page*60)
				clientSet.Do(func(client *http.Client) ClientState {
					bs, err := getBytes(client, url)
					if err != nil {
						tc.Log(sp("get bytes error %v", err))
						return Bad
					}
					jstr, err := GetPageConfigJson(bs)
					if err != nil {
						tc.Log(sp("get page config error %v", err))
						return Bad
					}
					var config PageConfig
					if err := json.Unmarshal(jstr, &config); err != nil {
						tc.Log(sp("unmarshal page config error %v", err))
						return Bad
					}
					// get items
					if config.Mods["itemlist"].Status == "hide" { // no items
						tc.Log("no items found")
						backend.AddItems([]Item{}, Job{
							Cat:  job.Cat,
							Page: job.Page,
						})
						return Good
					}
					items, err := GetItems(config.Mods["itemlist"].Data)
					if err != nil {
						tc.Log(sp("get items error %v", err))
						return Bad
					}
					// save items
					err = backend.AddItems(items, Job{
						Cat:  job.Cat,
						Page: job.Page,
					})
					ce(err, "save items")
					atomic.AddUint64(&itemsCollected, uint64(len(items)))
					// add next pass cat
					if len(items) > 0 && job.Page < 99 {
						wg.Add(1)
						jobsIn <- Job{
							Cat:  job.Cat,
							Page: job.Page + 1,
						}
					}
					return Good
				})
			}()
		}
	}()

	wg.Wait()
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

var ShopLevels = map[string]uint8{
	"icon-supple-level-jinguan": 0,
	"icon-supple-level-guan":    1,
	"icon-supple-level-zuan":    2,
	"icon-supple-level-xin":     3,
}

func GetItems(data []byte) (items []Item, err error) {
	defer ct(&err)
	var itemData struct {
		PostFeeText, Trace          string
		Auctions, RecommendAuctions []RawItem
		IsSameStyleView             bool
		Sellers                     []interface{}
		Query                       string
	}
	err = json.Unmarshal(data, &itemData)
	ce(err, "unmarshal")
	for _, raw := range itemData.Auctions {
		nid, err := strconv.Atoi(raw.Nid)
		ce(err, "nid strconv")

		cat, err := strconv.Atoi(raw.Category)
		ce(err, "category strconv")

		price := new(big.Rat)
		_, err = fmt.Sscan(raw.View_price, price)
		ce(err, "get price")

		salesStr := raw.View_sales
		salesStr = strings.Replace(salesStr, "人收货", "", -1)
		salesStr = strings.Replace(salesStr, "人付款", "", -1)
		sales, err := strconv.Atoi(salesStr)
		ce(err, "get sales")

		comments := 0
		if len(raw.Comment_count) > 0 {
			comments, err = strconv.Atoi(raw.Comment_count)
			ce(err, "comments strconv")
		}

		seller, err := strconv.Atoi(raw.User_id)
		ce(err, "get seller id")

		var levels []uint8
		for _, level := range raw.Shopcard.LevelClasses {
			if _, ok := ShopLevels[level.LevelClass]; !ok {
				panic(sp("%s not in ShopLevels", level.LevelClass))
			}
			levels = append(levels, ShopLevels[level.LevelClass])
		}

		item := Item{
			Nid: nid,
			Item1: Item1{
				Category: cat,
				Price:    price,
				Sales:    sales,
				Seller:   seller,
			},
			Item2: Item2{
				Title:    raw.Title,
				Location: raw.Item_loc,
			},
			Item3: Item3{
				Comments:          comments,
				SellerName:        raw.Nick,
				SellerEncryptedId: raw.Shopcard.EncryptedUserId,
				SellerLevels:      levels,
				SellerIsTmall:     raw.Shopcard.IsTmall,
				SellerCredit:      raw.Shopcard.SellerCredit,
			},
		}
		items = append(items, item)
	}
	return
}
