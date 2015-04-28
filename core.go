package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
)

var (
	lastFaceID = uint64(255)
	faces      = make(map[uint64]*face)

	reqSend    = make(chan *request)
	faceCreate = make(chan net.Conn)
	faceClose  = make(chan uint64)

	nextHop = newFIB()

	serializer = loopChecker(mux.Cacher(mux.HandlerFunc(
		func(w mux.Sender, i *ndn.Interest) {
			reqSend <- &request{
				sender:   w,
				interest: i,
			}
		})))
)

type request struct {
	sender   mux.Sender
	interest *ndn.Interest
}

func newFaceID() (id uint64) {
	lastFaceID++
	return lastFaceID
}

func run() {
	handleLocal()

	log("start")

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case conn := <-faceCreate:
			addFace(conn)
		case faceID := <-faceClose:
			removeFace(faceID)
		case req := <-reqSend:
			nextHop.ServeNDN(req.sender, req.interest)
		case <-quit:
			log("goodbye")
			return
		}
	}
}

func loopChecker(next mux.Handler) mux.Handler {
	forwarded := make(map[string]struct{})
	var mu sync.Mutex
	return mux.HandlerFunc(func(w mux.Sender, i *ndn.Interest) {
		interestID := fmt.Sprintf("%s/%x", i.Name, i.Nonce)
		mu.Lock()
		if _, ok := forwarded[interestID]; ok {
			mu.Unlock()
			return
		}
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
	interestRecv := make(chan *ndn.Interest)

	f := &face{
		Face: ndn.NewFace(conn, interestRecv),

		id:    newFaceID(),
		route: make(map[string]ndn.Route),
	}
	faces[f.id] = f

	go func() {
		for i := range interestRecv {
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
		nextHop.remove(name, f, true)
	}
}

func log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[core] %s", fmt.Sprintln(i...))
}
