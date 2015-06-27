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
	flags   map[string]struct{}
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
		flags:   make(map[string]struct{}),
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

func (s *TraceSet) Dump(w io.Writer, specs ...interface{}) {
	s.RLock()
	traces := make(Traces, len(s.traces))
	copy(traces, s.traces)
	s.RUnlock()
	var each func(*Trace)
	for _, spec := range specs {
		switch spec := spec.(type) {
		case func(*Trace) *Trace: // map
			traces = traces.Map(spec)
		case func(*Trace) bool: // filter
			traces = traces.Filter(spec)
		case func(*Trace, *Trace) bool: // sort
			traces.Sort(spec)
		case func(*Trace): // each
			each = spec
		default: // invalid
			panic(sp("invalid spec type %T", spec))
		}
	}
	if each == nil {
		each = func(trace *Trace) {
			fw(w, trace.what)
			for _, entry := range trace.Entries() {
				fw(w, entry.Message)
			}
			fw(w, "\n\n")
		}
	}
	traces.Each(each)
}

func (t *Trace) Entries() []*Entry {
	t.RLock()
	entries := make([]*Entry, len(t.entries))
	copy(entries, t.entries)
	t.RUnlock()
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

func (t *Trace) SetFlag(flag string) {
	t.Lock()
	t.flags[flag] = struct{}{}
	t.Unlock()
}

func (t *Trace) ClearFlag(flag string) {
	t.Lock()
	delete(t.flags, flag)
	t.Unlock()
}

func (t *Trace) Flag(flag string) (ret bool) {
	t.Lock()
	_, ret = t.flags[flag]
	t.Unlock()
	return
}
