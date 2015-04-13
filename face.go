package main

import (
	"fmt"

	"github.com/go-ndn/ndn"
)

type face struct {
	*ndn.Face

	id    uint64
	route map[string]ndn.Route
}

func (f *face) log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[%s] %s", f.RemoteAddr(), fmt.Sprintln(i...))
}

func (f *face) handle(req *request) {
	req.resp <- f.SendInterest(req.interest)
	f.log("forward", req.interest.Name)
}
