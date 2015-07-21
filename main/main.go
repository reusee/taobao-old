package main

import (
	"net/http"
	"os"
	"time"

	"github.com/reusee/hcutil"
	"github.com/reusee/taobao"
)

func init() {
	hcutil.DefaultRetryCount = 1
	hcutil.DefaultRetryInterval = time.Millisecond * 200

	taobao.StartWeb()
	go http.ListenAndServe("127.0.0.1:9991", nil)
}

func main() {
	backend, err := taobao.NewFileBackend()
	if err != nil {
		panic(err)
	}
	defer backend.Close()

	/*
		backend, err := NewMysql()
		ce(err, "new backend")
		defer backend.Close()
	*/

	switch os.Args[1] {
	case "collect":
		taobao.Collect(backend)
	case "stats":
		backend.Stats()
	case "fgcats":
		taobao.CollectForegroundCategories(backend)
	case "bgcats":
		taobao.CollectBackgroundCategories(backend)

	case "foo":
		backend.Foo()
	}
}
