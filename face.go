package main

import (
	"fmt"

	"github.com/go-ndn/ndn"
)

type face struct {
	*ndn.Face
	reqRecv chan<- *req

	id    uint64
	route map[string]ndn.Route
}

func (f *face) log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[%s] %s", f.RemoteAddr(), fmt.Sprintln(i...))
}

type req struct {
	sender   *face
	interest *ndn.Interest
	resp     chan<- (<-chan *ndn.Data)
}
