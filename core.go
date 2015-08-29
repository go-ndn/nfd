package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"

	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
)

var (
	lastFaceID = uint64(255)
	faces      = make(map[uint64]*face)

	key       ndn.Key
	timestamp uint64

	reqSend    = make(chan *request)
	faceCreate = make(chan net.Conn)
	faceClose  = make(chan uint64)

	nextHop = newFIB()

	serializer = loopChecker(mux.Cacher(mux.HandlerFunc(
		// serialize requests
		func(w ndn.Sender, i *ndn.Interest) {
			reqSend <- &request{
				Sender:   w,
				Interest: i,
			}
		})))
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
	handleLocal()

	if !*debug {
		log.SetOutput(ioutil.Discard)
	}
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

	if *debug {
		f.Logger = log.New(os.Stdout, fmt.Sprintf("[%s] ", conn.RemoteAddr()), log.LstdFlags)
	} else {
		f.Logger = log.New(ioutil.Discard, "", 0)
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
		nextHop.remove(name, f)
	}
}
