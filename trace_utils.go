package taobao

import (
	"math/rand"
	"sort"
)

type Traces []*Trace

func (s Traces) Reduce(initial interface{}, fn func(value interface{}, elem *Trace) interface{}) (ret interface{}) {
	ret = initial
	for _, elem := range s {
		ret = fn(ret, elem)
	}
	return
}

func (s Traces) Map(fn func(*Trace) *Trace) (ret Traces) {
	for _, elem := range s {
		ret = append(ret, fn(elem))
	}
	return
}

func (s Traces) Filter(filter func(*Trace) bool) (ret Traces) {
	for _, elem := range s {
		if filter(elem) {
			ret = append(ret, elem)
		}
	}
	return
}

func (s Traces) All(predict func(*Trace) bool) (ret bool) {
	ret = true
	for _, elem := range s {
		ret = predict(elem) && ret
	}
	return
}

func (s Traces) Any(predict func(*Trace) bool) (ret bool) {
	for _, elem := range s {
		ret = predict(elem) || ret
	}
	return
}

func (s Traces) Each(fn func(e *Trace)) {
	for _, elem := range s {
		fn(elem)
	}
}

func (s Traces) Shuffle() {
	for i := len(s) - 1; i >= 1; i-- {
		j := rand.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}

func (s Traces) Sort(cmp func(a, b *Trace) bool) {
	sorter := sliceSorter{
		l: len(s),
		less: func(i, j int) bool {
			return cmp(s[i], s[j])
		},
		swap: func(i, j int) {
			s[i], s[j] = s[j], s[i]
		},
	}
	_ = sorter.Len
	_ = sorter.Less
	_ = sorter.Swap
	sort.Sort(sorter)
}

type sliceSorter struct {
	l    int
	less func(i, j int) bool
	swap func(i, j int)
}

func (t sliceSorter) Len() int {
	return t.l
}

func (t sliceSorter) Less(i, j int) bool {
	return t.less(i, j)
}

func (t sliceSorter) Swap(i, j int) {
	t.swap(i, j)
}

func (s Traces) Clone() (ret Traces) {
	ret = make([]*Trace, len(s))
	copy(ret, s)
	return
}
