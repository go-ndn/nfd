package main

import (
	"fmt"

	"github.com/go-ndn/mux"
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

func (f *face) ServeNDN(w mux.Sender, i *ndn.Interest) {
	go func() {
		d, ok := <-f.SendInterest(i)
		if !ok {
			return
		}
		f.log("receive", d.Name)
		w.SendData(d)
		ndn.ContentStore.Add(d)
	}()
}
