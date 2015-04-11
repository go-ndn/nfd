package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-ndn/exact"
	"github.com/go-ndn/lpm"
	"github.com/go-ndn/ndn"
)

var (
	lastFaceID = uint64(255)
	faces      = make(map[uint64]*face)

	faceCreate = make(chan net.Conn)
	reqSend    = make(chan *req)
	faceClose  = make(chan uint64)

	forwarded = exact.New()
	fib       = lpm.New()
)

func newFaceID() (id uint64) {
	lastFaceID++
	return lastFaceID
}

func run() {
	log("start")

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case conn := <-faceCreate:
			addFace(conn)
		case rq := <-reqSend:
			handleReq(rq)
		case faceID := <-faceClose:
			removeFace(faceID)
		case <-quit:
			log("goodbye")
			return
		}
	}
}

func addFace(conn net.Conn) {
	interestRecv := make(chan *ndn.Interest)
	done := make(chan struct{})
	reqRecv := make(chan *req)
	dataRecv := make(chan *ndn.Data)

	f := &face{
		Face:    ndn.NewFace(conn, interestRecv),
		reqRecv: reqRecv,

		id:    newFaceID(),
		route: make(map[string]ndn.Route),
	}
	faces[f.id] = f

	// write
	go func() {
		for {
			select {
			case rq := <-reqRecv:
				ch := f.SendInterest(rq.interest)
				rq.resp <- ch
				close(rq.resp)
				f.log("forward", rq.interest.Name)
			case d := <-dataRecv:
				f.SendData(d)
			case <-done:
				return
			}
		}
	}()

	// read
	go func() {
		for i := range interestRecv {
			resp := make(chan (<-chan *ndn.Data))
			reqSend <- &req{
				sender:   f,
				interest: i,
				resp:     resp,
			}
			for ch := range resp {
				go func(ch <-chan *ndn.Data) {
					select {
					case d, ok := <-ch:
						if !ok {
							return
						}
						select {
						case dataRecv <- d:
						case <-done:
						}
					case <-done:
					}
				}(ch)
			}
		}
		faceClose <- f.id
		f.Close()
		close(done)
	}()
	f.log("face created")
}

func handleReq(rq *req) {
	fib.Match(rq.interest.Name.String(), func(v interface{}) {
		interestID := fmt.Sprintf("%s/%x", rq.interest.Name, rq.interest.Nonce)
		forwarded.Update(interestID, func(fw interface{}) interface{} {
			if fw == nil {
				for ch := range v.(map[chan<- *req]struct{}) {
					resp := make(chan (<-chan *ndn.Data))
					ch <- &req{
						interest: rq.interest,
						sender:   rq.sender,
						resp:     resp,
					}
					ret, ok := <-resp
					if ok {
						rq.resp <- ret
						break
					}
				}
				go func() {
					time.Sleep(time.Minute)
					forwarded.Update(interestID, func(interface{}) interface{} { return nil })
				}()
			} else {
				rq.sender.log("loop detected", interestID)
			}
			return struct{}{}
		})
	})
	close(rq.resp)
}

func removeFace(faceID uint64) {
	f := faces[faceID]
	delete(faces, faceID)
	for name := range f.route {
		removeNextHop(name, f.reqRecv)
	}
	f.log("face removed")
}

func addNextHop(name string, ch chan<- *req) {
	fib.Update(name, func(v interface{}) interface{} {
		var m map[chan<- *req]struct{}
		if v == nil {
			m = make(map[chan<- *req]struct{})
		} else {
			m = v.(map[chan<- *req]struct{})
		}
		m[ch] = struct{}{}
		return m
	}, false)
}

func removeNextHop(name string, ch chan<- *req) {
	fib.Update(name, func(v interface{}) interface{} {
		if v == nil {
			return nil
		}
		m := v.(map[chan<- *req]struct{})
		delete(m, ch)
		if len(m) == 0 {
			return nil
		}
		return m
	}, false)
}

func log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[core] %s", fmt.Sprintln(i...))
}
