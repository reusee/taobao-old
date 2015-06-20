package main

import (
	"fmt"
	"os"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/reusee/catch"
	"github.com/reusee/hcutil"
)

var (
	ce = catch.PkgChecker("taobao")
	ct = catch.Catch
	pt = fmt.Printf
	sp = fmt.Sprintf
)

func init() {
	hcutil.DefaultRetryCount = 1
	hcutil.DefaultRetryInterval = time.Millisecond * 200

	go http.ListenAndServe("127.0.0.1:9991", nil)
}

func main() {
	/*
		backend, err := NewMongo()
		ce(err, "new backend")
		defer backend.Close()
	*/
	var backend Backend

	switch os.Args[1] {
	case "collect":
		collect(backend)
	case "stats":
		backend.Stats()
	case "cats":
		collectCategories(backend)
	}
}

type Backend interface {
	AddJob(Job) error
	DoneJob(Job) error
	GetJobs() ([]Job, error)
	AddItem(Item, Job) error
	AddCat(Cat) error

	Stats()
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

type Job struct {
	Cat, Page int
	Done      bool
}

type Cat struct {
	Cat       int
	Name      string
	Relatives []int
}
