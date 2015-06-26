package main

import (
	"sync"
	"sync/atomic"
)

type TraceSet struct {
	sync.RWMutex
	enabled atomic.Value
	traces  []*Trace
}

type Trace struct {
	sync.RWMutex
	enabled *atomic.Value
	what    string
	entries []*Entry
}

type Entry struct {
	Message string
}

func NewTraceSet() *TraceSet {
	s := &TraceSet{}
	s.enabled.Store(true)
	return s
}

func (s *TraceSet) NewTrace(what string) *Trace {
	t := &Trace{
		enabled: &s.enabled,
		what:    what,
	}
	s.Lock()
	s.traces = append(s.traces, t)
	s.Unlock()
	return t
}

func (s *TraceSet) Enable() {
	s.enabled.Store(true)
}

func (s *TraceSet) Disable() {
	s.enabled.Store(false)
}

func (t *Trace) Log(msg string) {
	if !t.enabled.Load().(bool) {
		return
	}
	e := &Entry{
		Message: msg,
	}
	t.Lock()
	t.entries = append(t.entries, e)
	t.Unlock()
}
