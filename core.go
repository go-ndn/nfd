package main

import (
	"fmt"
	"net"
	"sync"
	"time"

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
		func(w ndn.Sender, i *ndn.Interest) {
			reqSend <- &request{
				sender:   w,
				interest: i,
			}
		})))
)

type request struct {
	sender   ndn.Sender
	interest *ndn.Interest
}

func newFaceID() (id uint64) {
	lastFaceID++
	return lastFaceID
}

func run() {
	handleLocal()

	log("start")

	for {
		select {
		case conn := <-faceCreate:
			addFace(conn)
		case faceID := <-faceClose:
			removeFace(faceID)
		case req := <-reqSend:
			nextHop.ServeNDN(req.sender, req.interest)
		}
	}
}

func loopChecker(next mux.Handler) mux.Handler {
	forwarded := make(map[string]struct{})
	var mu sync.RWMutex
	return mux.HandlerFunc(func(w ndn.Sender, i *ndn.Interest) {
		interestID := fmt.Sprintf("%s/%x", i.Name, i.Nonce)
		mu.RLock()
		_, ok := forwarded[interestID]
		mu.RUnlock()
		if ok {
			return
		}
		mu.Lock()
		forwarded[interestID] = struct{}{}
		mu.Unlock()
		go func() {
			time.Sleep(time.Minute)
			mu.Lock()
			delete(forwarded, interestID)
			mu.Unlock()
		}()
		next.ServeNDN(w, i)
	})
}

func addFace(conn net.Conn) {
	recv := make(chan *ndn.Interest)

	f := &face{
		Face: ndn.NewFace(conn, recv),

		id:    newFaceID(),
		route: make(map[string]ndn.Route),
	}
	faces[f.id] = f

	go func() {
		for i := range recv {
			serializer.ServeNDN(f, i)
		}
		faceClose <- f.id
		f.Close()
		f.log("face removed")
	}()
	f.log("face created")
}

func removeFace(faceID uint64) {
	f := faces[faceID]
	delete(faces, faceID)
	for name := range f.route {
		nextHop.remove(name, f)
	}
}

func log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[core] %s", fmt.Sprintln(i...))
}
