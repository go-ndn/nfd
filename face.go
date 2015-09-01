package main

import (
	"github.com/go-ndn/log"
	"github.com/go-ndn/ndn"
)

type face struct {
	ndn.Face
	log.Logger
	id    uint64
	route map[string]ndn.Route
}

func (f *face) ServeNDN(w ndn.Sender, i *ndn.Interest) {
	go func() {
		f.Println("forward", i.Name)
		d, ok := <-f.SendInterest(i)
		if !ok {
			return
		}
		f.Println("receive", d.Name)
		w.SendData(d)
	}()
}
