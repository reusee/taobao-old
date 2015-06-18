package main

import (
	"net/url"
	"sort"

	"gopkg.in/mgo.v2"

	"github.com/PuerkitoBio/goquery"
	"github.com/lib/pq"
)

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

func ignoreExistsColle(err error) error {
	if err, ok := err.(*mgo.QueryError); ok {
		if err.Message == "collection already exists" {
			return nil
		}
	}
	return err
}

func allowDup(err error) error {
	if err, ok := err.(*mgo.LastError); ok {
		if err.Code == 11000 {
			return nil
		}
	}
	return err
}

func allowDupTable(err error) error {
	if err, ok := err.(*pq.Error); ok {
		if err.Code.Name() == "duplicate_table" {
			return nil
		}
	}
	return err
}

func allowUniqVio(err error) error {
	if err, ok := err.(*pq.Error); ok {
		if err.Code.Name() == "unique_violation" {
			return nil
		}
	}
	return err
}
