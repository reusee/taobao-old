package taobao

import (
	"container/list"
	"net/http"
)

func NewClientsChan() (chan<- *http.Client, <-chan *http.Client, chan struct{}) {
	store := list.New()
	in := make(chan *http.Client)
	out := make(chan *http.Client)
	kill := make(chan struct{})

	go func() {
		defer func() {
			close(in)
			close(out)
		}()
		for {
			if store.Len() > 0 {
				e := store.Front().Value.(*http.Client)
				select {
				case out <- e:
					store.Remove(store.Front())
				case v := <-in:
					store.PushBack(v)
				case <-kill:
					return
				}
			} else {
				select {
				case v := <-in:
					store.PushBack(v)
				case <-kill:
					return
				}
			}
		}
	}()

	return in, out, kill
}
