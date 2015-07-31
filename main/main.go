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
	var closeFunc func()
	todayBackend := func() taobao.Backend {
		backend, err := taobao.NewFileBackend(time.Now())
		ce(err, "file backend")
		closeFunc = func() {
			backend.Close()
		}
		return backend
	}
	backendByDate := func(t time.Time) taobao.Backend {
		backend, err := taobao.NewFileBackend(t)
		ce(err, "file backend")
		closeFunc = func() {
			backend.Close()
		}
		return backend
	}

	/*
		backend, err := NewMysql()
		ce(err, "new backend")
		defer backend.Close()
	*/

	switch os.Args[1] {
	case "collect":
		backend := todayBackend()
		taobao.Collect(backend)
	case "stats":
		backend := todayBackend()
		backend.Stats()
	case "fgcats":
		backend := todayBackend()
		taobao.CollectForegroundCategories(backend)
	case "bgcats":
		backend := todayBackend()
		taobao.CollectBackgroundCategories(backend)
	case "foo":
		now, err := time.Parse("2006-01-02", os.Args[2])
		ce(err, "parse date")
		pt("%v\n", now)
		backend := backendByDate(now)
		backend.Foo()
	case "stat":
		now, err := time.Parse("2006-01-02", os.Args[2])
		ce(err, "parse date")
		pt("%v\n", now)
		backend := backendByDate(now)
		backend.Stats()
	}

	closeFunc()
}
