package main

import "sync"

var tracer *Tracer

func init() {
	tracer = NewTracer()
}

type Tracer struct {
	Traces      []*Trace
	EndedTraces []*Trace //TODO
	lock        *sync.Mutex
}

type Trace struct {
	ended  bool
	What   string
	Ticks  []Tick
	tracer *Tracer
}

type Tick struct {
	What string
}

func NewTracer() *Tracer {
	return &Tracer{
		lock: new(sync.Mutex),
	}
}

func (r *Tracer) Begin(what string) *Trace {
	t := &Trace{
		What:   what,
		tracer: r,
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	r.Traces = append(r.Traces, t)
	return t
}

func (t *Trace) Tick(what string) {
	t.Ticks = append(t.Ticks, Tick{
		What: what,
	})
}

func (t *Trace) End() {
	t.ended = true
}
