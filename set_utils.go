package taobao

type IntSet map[int]struct{}

func NewIntSet() IntSet {
	return IntSet(make(map[int]struct{}))
}

func (s IntSet) Add(t int) {
	s[t] = struct{}{}
}

func (s IntSet) Del(t int) {
	delete(s, t)
}

func (s IntSet) Has(t int) (ok bool) {
	_, ok = s[t]
	return
}

func (s IntSet) Each(fn func(int)) {
	for e := range s {
		fn(e)
	}
}

func (s IntSet) Slice() (ret []int) {
	for e := range s {
		ret = append(ret, e)
	}
	return
}
