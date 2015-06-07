package main

import "fmt"

type Err struct {
	Pkg  string
	Info string
	Err  error
}

func (e *Err) Error() string {
	return fmt.Sprintf("%s error: %s caused by\n%v", e.Pkg, e.Info, e.Err)
}

func makeErr(err error, info string) *Err {
	return &Err{
		Pkg:  `taobao`,
		Info: info,
		Err:  err,
	}
}
