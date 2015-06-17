package main

import (
	"fmt"
	"os"

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
	session, err := mgo.Dial("localhost")
	ce(err, "connect to db")
	defer session.Close()
	db := session.DB("taobao")

	switch os.Args[1] {
	case "collect":
		collect(db)
	case "stats":
		stats(db)
	}
}

type Item struct {
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
