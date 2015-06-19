package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"gopkg.in/mgo.v2"

	"github.com/reusee/catch"
)

var (
	ce = catch.PkgChecker("taobao")
	ct = catch.Catch
	pt = fmt.Printf
	sp = fmt.Sprintf
)

func main() {
	// database
	session, err := mgo.Dial("127.0.0.1")
	ce(err, "connect to db")
	defer session.Close()
	db := session.DB("taobao")

	now := time.Now()
	date := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())

	switch os.Args[1] {
	case "collect":
		collect(db, date)
	case "stats":
		stats(db, date)
	case "cats":
		collectCategories(http.DefaultClient)
	case "foo":
		foo(db, date)
	}
}

type Item struct {
	Sources []Source

	//I2iTags       map[string]interface{}
	Nid           string
	Category      string
	Pid           string
	Title         string
	Raw_title     string
	Pic_url       string
	Detail_url    string
	View_price    string
	View_fee      string
	Item_loc      string
	Reserve_price string
	View_sales    string
	Comment_count string
	User_id       string
	Nick          string
	Shopcard      struct {
		LevelClasses []struct {
			LevelClass string
		}
		IsTmall         bool
		Delivery        []int
		Description     []int
		Service         []int
		EncryptedUserId string
		SellerCredit    int
		TotalRate       int
	}
	//Icon        interface{}
	Comment_url string
	ShopLink    string
}

type Raw struct {
	Cat, Page int
	Items     []Item
	Html      []byte
}

type Source struct {
	Cat, Page int
}
