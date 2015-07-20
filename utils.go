package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

import "math/rand"

import crand "crypto/rand"

func dumpUrl(rawUrl string) {
	u, err := url.Parse(rawUrl)
	ce(err, "parse url")
	query := u.Query()
	var keys []string
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		pt("%s -> %v\n", k, query[k])
	}
}

func dumpDoc(doc *goquery.Document) {
	html, _ := doc.Html()
	pt("%s\n", html)
}

func getBytes(client *http.Client, url string) (ret []byte, err error) {
	defer ct(&err)
	resp, err := client.Get(url)
	ce(err, "get")
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	ce(err, "read")
	return buf.Bytes(), nil
}

func lp() {
	if p := recover(); p != nil {
		pt("%v\n", p)
	}
}

type Jobs []Job

func (s Jobs) Shuffle() {
	for i := len(s) - 1; i >= 1; i-- {
		j := rand.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}

func init() {
	var seed int64
	binary.Read(crand.Reader, binary.LittleEndian, &seed)
	rand.Seed(seed)
}

func (s Jobs) Sort(cmp func(a, b Job) bool) {
	sorter := sliceSorter{
		l: len(s),
		less: func(i, j int) bool {
			return cmp(s[i], s[j])
		},
		swap: func(i, j int) {
			s[i], s[j] = s[j], s[i]
		},
	}
	_ = sorter.Len
	_ = sorter.Less
	_ = sorter.Swap
	sort.Sort(sorter)
}

type StrSet map[string]struct{}

func NewStrSet() StrSet {
	return StrSet(make(map[string]struct{}))
}

func (s StrSet) Add(t string) {
	s[t] = struct{}{}
}

func (s StrSet) Del(t string) {
	delete(s, t)
}

func (s StrSet) Has(t string) (ok bool) {
	_, ok = s[t]
	return
}

func (s StrSet) Each(fn func(string)) {
	for e := range s {
		fn(e)
	}
}

func (s StrSet) Slice() (ret []string) {
	for e := range s {
		ret = append(ret, e)
	}
	return
}

func withLock(l sync.Locker, fn func()) {
	l.Lock()
	fn()
	l.Unlock()
}
