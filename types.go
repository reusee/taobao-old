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
	GetBgCatInfo(int) (CatInfo, error)
	SetBgCatInfo(int, CatInfo) error

	Stats()
	Foo()

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
	Title             string
	Price             *big.Rat
	Location          string
	Sales             int
	Comments          int
	Seller            int
	SellerEncryptedId string
	SellerName        string
	SellerLevels      []uint8
	SellerIsTmall     bool
	SellerCredit      int
}

type Raw struct {
	Cat, Page int
	Items     []RawItem
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
