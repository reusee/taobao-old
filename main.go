package main

import (
	"fmt"
	"os"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/reusee/hcutil"
)

var (
	pt = fmt.Printf
	sp = fmt.Sprintf
	fw = fmt.Fprintf
)

func init() {
	hcutil.DefaultRetryCount = 1
	hcutil.DefaultRetryInterval = time.Millisecond * 200

	web()
	go http.ListenAndServe("127.0.0.1:9991", nil)
}

func main() {
	backend, err := NewFileBackend()
	ce(err, "new file backend")
	defer backend.Close()

	/*
		backend, err := NewMysql()
		ce(err, "new backend")
		defer backend.Close()
	*/

	switch os.Args[1] {
	case "collect":
		collect(backend)
	case "stats":
		backend.Stats()
	case "fgcats":
		collectForegroundCategories(backend)
	case "bgcats":
		collectBackgroundCategories(backend)

	case "foo":
		backend.Foo()
	}
}

type Backend interface {
	// jobs
	AddJobs([]Job) error
	DoneJob(Job) error
	GetJobs() ([]Job, error)

	// items
	AddItems([]Item, Job) error

	// fgcats
	AddFgCat(Cat) error
	GetFgCats() ([]Cat, error)

	// bgcats
	AddBgCat(Cat) error
	GetBgCatInfo(int) (CatInfo, error)
	SetBgCatInfo(int, CatInfo) error

	Stats()
	Foo()

	LogClient(ClientInfo, ClientState)
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
	Parent    int
}

type CatInfo struct {
	LastChecked time.Time
}

type NavData struct {
	Common []struct {
		Text string
		Sub  []struct {
			Text  string
			Key   string
			Value string
		}
	}
	Breadcrumbs struct {
		BaobeiTotalHit string
		Catpath        []struct {
			Catid string
			Name  string
		}
	}
	Hidenav bool
}
