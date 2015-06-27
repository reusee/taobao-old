package main

import (
	"io"
	"sync"
	"sync/atomic"
)

type TraceSet struct {
	sync.RWMutex
	enabled atomic.Value
	traces  Traces
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

func (s *TraceSet) Dump(w io.Writer, fns ...interface{}) {
	s.RLock()
	traces := make(Traces, len(s.traces))
	copy(traces, s.traces)
	s.RUnlock()
	for _, fn := range fns {
		switch fn := fn.(type) {
		case func(*Trace) *Trace: // map
			traces = traces.Map(fn)
		case func(*Trace) bool: // filter
			traces = traces.Filter(fn)
		case func(*Trace, *Trace) bool: // sort
			traces.Sort(fn)
		default: // invalid
			panic(sp("invalid function type %T", fn))
		}
	}
	for _, trace := range traces {
		fw(w, trace.what)
		trace.RLock()
		entries := make([]*Entry, len(trace.entries))
		copy(entries, trace.entries)
		trace.RUnlock()
		for _, entry := range entries {
			fw(w, entry.Message)
		}
		fw(w, "\n")
	}
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
