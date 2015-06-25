package main

import (
	"sync"
	"sync/atomic"
)

var tracer *Tracer

func init() {
	tracer = NewTracer()
}

type Tracer struct {
	enabled     atomic.Value
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
	t := &Tracer{
		lock: new(sync.Mutex),
	}
	t.enabled.Store(false)
	return t
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

func (r *Tracer) Enable() {
	r.enabled.Store(true)
}

func (r *Tracer) Disable() {
	r.enabled.Store(false)
}

func (t *Trace) Tick(what string) {
	if !t.tracer.enabled.Load().(bool) {
		return
	}
	t.Ticks = append(t.Ticks, Tick{
		What: what,
	})
}

func (t *Trace) End() {
	t.ended = true
}
