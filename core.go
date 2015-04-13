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

			cache := getCache(i)
			if cache != nil {
				f.SendData(cache)
				continue
			}

			// forward
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
						f.SendData(d)
						addCache(d)
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

func addCache(d *ndn.Data) {
	ndn.ContentStore.Update(d.Name.String(), func(v interface{}) interface{} {
		var m map[*ndn.Data]time.Time
		if v == nil {
			m = make(map[*ndn.Data]time.Time)
		} else {
			m = v.(map[*ndn.Data]time.Time)
		}
		m[d] = time.Now()
		return m
	})
}

func getCache(i *ndn.Interest) (cache *ndn.Data) {
	ndn.ContentStore.Match(i.Name.String(), func(v interface{}) {
		if v == nil {
			return
		}
		name := i.Name.String()
		for d, t := range v.(map[*ndn.Data]time.Time) {
			if i.Selectors.Match(name, d, t) {
				cache = d
				break
			}
		}
	})
	return
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

func handleReq(rq *req) {
	fib.Match(rq.interest.Name.String(), func(v interface{}) {
		for h := range v.(map[handler]struct{}) {
			h.handleReq(rq)
			break
		}
	})
	close(rq.resp)
}

func removeFace(faceID uint64) {
	f := faces[faceID]
	delete(faces, faceID)
	for name := range f.route {
		removeNextHop(name, f)
	}
	f.log("face removed")
}

func addNextHop(name string, h handler) {
	fib.Update(name, func(v interface{}) interface{} {
		var m map[handler]struct{}
		if v == nil {
			m = make(map[handler]struct{})
		} else {
			m = v.(map[handler]struct{})
		}
		m[h] = struct{}{}
		return m
	}, false)
}

func removeNextHop(name string, h handler) {
	fib.Update(name, func(v interface{}) interface{} {
		if v == nil {
			return nil
		}
		m := v.(map[handler]struct{})
		delete(m, h)
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
