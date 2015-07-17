package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/http"
	"net/url"
	"sort"

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
