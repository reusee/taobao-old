package main

import "net/http"

func web() {
	http.Handle("/", http.FileServer(http.Dir("./web")))
}
