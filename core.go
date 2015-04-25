package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-ndn/ndn"
)

var (
	lastFaceID = uint64(255)
	faces      = make(map[uint64]*face)

	reqSend    = make(chan *request)
	faceCreate = make(chan net.Conn)
	faceClose  = make(chan uint64)

	forwarded = make(map[string]struct{})
	mu        sync.Mutex

	nextHop = newFIB()
)

type request struct {
	sender   *face
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

func checkLoop(i *ndn.Interest) bool {
	mu.Lock()
	defer mu.Unlock()
	interestID := fmt.Sprintf("%s/%x", i.Name, i.Nonce)
	if _, ok := forwarded[interestID]; ok {
		return true
	}
	forwarded[interestID] = struct{}{}
	go func() {
		time.Sleep(time.Minute)
		mu.Lock()
		delete(forwarded, interestID)
		mu.Unlock()
	}()
	return false
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
			if checkLoop(i) {
				continue
			}

			cache := ndn.ContentStore.Get(i)
			if cache != nil {
				f.SendData(cache)
				continue
			}

			reqSend <- &request{
				sender:   f,
				interest: i,
			}
		}
		faceClose <- f.id
		f.Close()
	}()
	f.log("face created")
}

func removeFace(faceID uint64) {
	f := faces[faceID]
	delete(faces, faceID)
	for name := range f.route {
		nextHop.remove(name, f, true)
	}
	f.log("face removed")
}

func log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[core] %s", fmt.Sprintln(i...))
}
