package main

import "github.com/go-ndn/ndn"

type req struct {
	sender   *face
	interest *ndn.Interest
	resp     chan<- (<-chan *ndn.Data)
}

type handler interface {
	handleReq(*req)
}
