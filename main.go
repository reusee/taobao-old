package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
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
	db, err := sql.Open("postgres", "user=reus dbname=taobao sslmode=disable")
	ce(err, "connect to db")
	defer db.Close()
	db.SetMaxIdleConns(64)

	now := time.Now()
	date := sp("%04d%02d%02d", now.Year(), now.Month(), now.Day())
	checkSchema(db, date)

	switch os.Args[1] {
	case "collect":
		collect(db, date)
	case "stats":
		stats(db, date)
	case "cats":
		collectCategories(http.DefaultClient)
	case "foo":
		//foo(db) //TODO
	}
}

type Item struct {
	Cats []int

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
