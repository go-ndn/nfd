package main

import (
	"fmt"

	"github.com/go-ndn/ndn"
)

type face struct {
	ndn.Face

	id    uint64
	route map[string]ndn.Route
}

func (f *face) log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[%s] %s", f.RemoteAddr(), fmt.Sprintln(i...))
}

func (f *face) ServeNDN(w ndn.Sender, i *ndn.Interest) {
	go func() {
		f.log("forward", i.Name)
		d, ok := <-f.SendInterest(i)
		if !ok {
			return
		}
		f.log("receive", d.Name)
		w.SendData(d)
	}()
}
