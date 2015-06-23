package main

type Trace struct {
	What  string
	Ticks []Tick
}

type Tick struct {
	What string
}

func NewTrace(what string) *Trace {
	return &Trace{
		What: what,
	}
}

func (t *Trace) Tick(what string) {
	t.Ticks = append(t.Ticks, Tick{
		What: what,
	})
}

func (t *Trace) Done(handler func(*Trace)) {
	if handler != nil {
		handler(t)
	}
}
