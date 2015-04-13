package main

import "github.com/go-ndn/ndn"

type request struct {
	sender   *face
	interest *ndn.Interest
	resp     chan<- (<-chan *ndn.Data)
}

type handler interface {
	handle(*request)
}
