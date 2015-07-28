package taobao

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sort"
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

	// first pass
	cats := []int{}
	fgcats, err := backend.GetFgCats()
	ce(err, "get fgcats")
	for _, cat := range fgcats {
		cats = append(cats, cat.Cat)
	}
	pt("%d fgcats\n", len(cats))

	for page := 0; page < 100; page++ {
		sort.Ints(cats)
		wg := new(sync.WaitGroup)
		wg.Add(len(cats))
		sem := make(chan struct{}, 256)
		nextPassCats := []int{}
		lock := new(sync.Mutex)
		done := make(chan struct{})
		var c1, c2 uint64
		go func() {
			ticker := time.NewTicker(time.Second * 5)
			for {
				select {
				case <-ticker.C:
					pt("%d - %d / %d / %d\n", page, atomic.SwapUint64(&c1, 0), atomic.LoadUint64(&c2), len(cats))
				case <-done:
					return
				}
			}
		}()
		for _, cat := range cats {
			sem <- struct{}{}
			cat := cat
			go func() {
				defer func() {
					wg.Done()
					atomic.AddUint64(&c1, 1)
					atomic.AddUint64(&c2, 1)
					<-sem
				}()
				// check
				if backend.IsCollected(Job{
					Cat:  cat,
					Page: page,
				}) {
					withLock(lock, func() {
						nextPassCats = append(nextPassCats, cat)
					})
					return
				}
				// trace
				tc := jobTraceSet.NewTrace(sp("cat %d, page %d", cat, page))
				defer tc.SetFlag("done")
				url := sp("http://s.taobao.com/list?cat=%d&sort=sale-desc&bcoffset=0&s=%d", cat, page*60)
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
							Cat:  cat,
							Page: page,
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
						Cat:  cat,
						Page: page,
					})
					ce(err, "save items")
					// add next pass cat
					if len(items) > 0 {
						withLock(lock, func() {
							nextPassCats = append(nextPassCats, cat)
						})
					}
					return Good
				})
			}()
		}
		wg.Wait()
		pt("page %d collected\n", page)
		close(done)
		cats = nextPassCats
	}
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
			Nid:               nid,
			Category:          cat,
			Title:             raw.Title,
			Price:             price,
			Location:          raw.Item_loc,
			Sales:             sales,
			Comments:          comments,
			Seller:            seller,
			SellerName:        raw.Nick,
			SellerEncryptedId: raw.Shopcard.EncryptedUserId,
			SellerLevels:      levels,
			SellerIsTmall:     raw.Shopcard.IsTmall,
			SellerCredit:      raw.Shopcard.SellerCredit,
		}
		items = append(items, item)
	}
	return
}
