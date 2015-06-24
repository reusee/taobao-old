package main

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 1024,
}

func web() {
	http.Handle("/", http.FileServer(http.Dir("./web")))

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		defer lp()
		conn, err := upgrader.Upgrade(w, r, nil)
		ce(err, "upgrade")
		/*
			for {
				what, msg, err := conn.ReadMessage()
				if err == io.EOF {
					return
				}
				ce(err, "read")
				if what == websocket.CloseMessage {
					return
				}
				pt("%d %s\n", what, msg)
				conn.WriteMessage(what, msg)
			}
		*/
		for {
			ce(conn.WriteMessage(websocket.TextMessage, []byte("foo")), "write")
			time.Sleep(time.Second)
		}
	})
}
