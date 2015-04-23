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
	reqSend    = make(chan *request)
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
		case req := <-reqSend:
			handle(req)
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
	stop := make(chan struct{})

	f := &face{
		Face: ndn.NewFace(conn, interestRecv),

		id:    newFaceID(),
		route: make(map[string]ndn.Route),
	}
	faces[f.id] = f

	go func() {
		for i := range interestRecv {
			// detect loop
			interestID := fmt.Sprintf("%s/%x", i.Name, i.Nonce)
			if checkLoop(interestID) {
				f.log("loop detected", interestID)
				continue
			}

			cache := ndn.ContentStore.Get(i)
			if cache != nil {
				f.SendData(cache)
				continue
			}

			// forward
			resp := make(chan (<-chan *ndn.Data))
			reqSend <- &request{
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
						f.log("receive", d.Name)
						f.SendData(d)
						ndn.ContentStore.Add(d)
					case <-stop:
					}
				}(ch)
			}
		}
		faceClose <- f.id
		f.Close()
		close(stop)
	}()
	f.log("face created")
}

func checkLoop(interestID string) (loop bool) {
	forwarded.Update(interestID, func(fw interface{}) interface{} {
		if fw == nil {
			go func() {
				time.Sleep(time.Minute)
				forwarded.Update(interestID, func(interface{}) interface{} { return nil })
			}()
		} else {
			loop = true
		}
		return struct{}{}
	})
	return
}

func handle(req *request) {
	fib.Match(req.interest.Name.String(), func(v interface{}) {
		for h := range v.(map[handler]struct{}) {
			h.handle(req)
			break
		}
	})
	close(req.resp)
}

func removeFace(faceID uint64) {
	f := faces[faceID]
	delete(faces, faceID)
	for name := range f.route {
		removeNextHop(name, f, true)
	}
	f.log("face removed")
}

func addNextHop(name string, h handler, childInherit bool) {
	updater := func(v interface{}) interface{} {
		var m map[handler]struct{}
		if v == nil {
			m = make(map[handler]struct{})
		} else {
			m = v.(map[handler]struct{})
		}
		m[h] = struct{}{}
		return m
	}
	if childInherit {
		fib.UpdateAll(name, func(_ string, v interface{}) interface{} {
			return updater(v)
		}, false)
	} else {
		fib.Update(name, updater, false)
	}
}

func removeNextHop(name string, h handler, childInherit bool) {
	updater := func(v interface{}) interface{} {
		if v == nil {
			return nil
		}
		m := v.(map[handler]struct{})
		delete(m, h)
		if len(m) == 0 {
			return nil
		}
		return m
	}
	if childInherit {
		fib.UpdateAll(name, func(_ string, v interface{}) interface{} {
			return updater(v)
		}, false)
	} else {
		fib.Update(name, updater, false)
	}
}

func log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[core] %s", fmt.Sprintln(i...))
}
