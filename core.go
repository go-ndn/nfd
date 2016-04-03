package main

import (
	"fmt"
	"net"

	"github.com/go-ndn/log"
	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
)

var (
	lastFaceID uint64
	faces      = make(map[uint64]*face)

	key       ndn.Key
	timestamp uint64

	reqSend    = make(chan *request)
	faceCreate = make(chan net.Conn)
	faceClose  = make(chan uint64)

	nextHop *fib

	// serialize requests
	serializer = mux.HandlerFunc(func(w ndn.Sender, i *ndn.Interest) {
		reqSend <- &request{
			Sender:   w,
			Interest: i,
		}
	})
)

type request struct {
	ndn.Sender
	*ndn.Interest
}

func newFaceID() (id uint64) {
	lastFaceID++
	return lastFaceID
}

func run() {
	nextHop = newFIB()
	handleLocal()

	for {
		select {
		case conn := <-faceCreate:
			addFace(conn)
		case faceID := <-faceClose:
			removeFace(faceID)
		case req := <-reqSend:
			nextHop.ServeNDN(req.Sender, req.Interest)
		}
	}
}

func addFace(conn net.Conn) {
	recv := make(chan *ndn.Interest)

	f := &face{
		Face:  ndn.NewFace(conn, recv),
		id:    newFaceID(),
		route: make(map[string]ndn.Route),
	}

	if *flagDebug {
		f.Logger = log.New(log.Stderr, fmt.Sprintf("[%s] ", conn.RemoteAddr()))
	} else {
		f.Logger = log.Discard
	}

	faces[f.id] = f

	go func() {
		for i := range recv {
			serializer.ServeNDN(f, i)
		}
		faceClose <- f.id
		f.Close()
		f.Println("face removed")
	}()
	f.Println("face created")
}

func removeFace(faceID uint64) {
	f := faces[faceID]
	delete(faces, faceID)
	for name := range f.route {
		nextHop.remove(ndn.NewName(name), faceID)
	}
}
