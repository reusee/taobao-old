package taobao

import (
	"math/big"
	"time"
)

type Backend interface {
	// job
	IsCollected(Job) bool

	// items
	AddItems([]Item, Job) error

	// fgcats
	AddFgCat(Cat) error
	GetFgCats() ([]Cat, error)

	// bgcats
	AddBgCat(Cat) error
	GetBgCatLastUpdated(int) (time.Time, error)
	SetBgCatLastUpdated(int, time.Time) error

	Stats()
	PostProcess()

	LogClient(ClientInfo, ClientState)
}

type RawItem struct {
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

type Item struct {
	Nid               int
	Category          int
	Price             *big.Rat
	Sales             int
	Seller            int
	Title             string
	Location          string
	Comments          int
	SellerEncryptedId string
	SellerName        string
	SellerLevels      []uint8
	SellerIsTmall     bool
	SellerCredit      int
}

type EntryHeader struct {
	Cat     uint64
	Page    uint8
	NidsLen uint32
	Len1    uint32
	Len2    uint32
	Len3    uint32
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

type CatStat struct {
	Items int
	Sales int
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

type BgCat struct {
	Cat         int
	Name        string
	Subs        IntSet
	LastUpdated time.Time
}
