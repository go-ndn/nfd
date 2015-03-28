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
	"github.com/go-ndn/tlv"
)

var (
	lastFaceID = uint64(255)
	faces      = make(map[uint64]*face)

	faceCreate = make(chan net.Conn)
	reqSend    = make(chan *req)
	faceClose  = make(chan *face)

	forwarded = exact.New()
	fib       = lpm.New()
)

func newFaceID() (id uint64) {
	lastFaceID++
	return lastFaceID
}

func log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[core] %s", fmt.Sprintln(i...))
}

func handleLocal() {
	for _, rt := range localRoute {
		reqRecv := make(chan *req)
		addNextHop(lpm.Key(rt.url), reqRecv)
		go func(rt route) {
			for {
				rq := <-reqRecv
				var (
					v interface{}
					t uint64
				)
				if rt.handleCommand != nil {
					// command
					t = 101
					cmd := new(ndn.Command)
					tlv.Copy(&rq.interest.Name, cmd)
					if cmd.Timestamp <= timestamp || key.Verify(cmd, cmd.SignatureValue.SignatureValue) != nil {
						v = respNotAuthorized
						goto REQ_DONE
					}
					timestamp = cmd.Timestamp
					params := &cmd.Parameters.Parameters

					var f *face
					if params.FaceID == 0 {
						f = rq.sender
					} else {
						var ok bool
						f, ok = faces[params.FaceID]
						if !ok {
							v = respIncorrectParams
							goto REQ_DONE
						}
					}

					v = rt.handleCommand(params, f)
				} else {
					// dataset
					t = 128
					v = rt.handleDataset()
				}

			REQ_DONE:
				d := &ndn.Data{Name: rq.interest.Name}
				d.Content, _ = tlv.MarshalByte(v, t)
				ch := make(chan *ndn.Data, 1)
				ch <- d
				close(ch)
				rq.resp <- ch
				close(rq.resp)
			}
		}(rt)
	}
}

func run() {
	log("start")

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case conn := <-faceCreate:
			ch := make(chan *ndn.Interest)
			f := &face{
				Face:         ndn.NewFace(conn, ch),
				reqRecv:      make(chan *req),
				interestRecv: ch,

				id:    newFaceID(),
				route: make(map[string]ndn.Route),
			}
			faces[f.id] = f
			f.log("face created")
			go f.run()
		case rq := <-reqSend:
			handleReq(rq)
		case f := <-faceClose:
			delete(faces, f.id)
			for name := range f.route {
				removeNextHop(ndn.NewName(name), f.reqRecv)
			}
			f.log("face removed")
		case <-quit:
			log("goodbye")
			return
		}
	}
}

func addNextHop(name fmt.Stringer, ch chan<- *req) {
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

func removeNextHop(name fmt.Stringer, ch chan<- *req) {
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

func handleReq(rq *req) {
	defer close(rq.resp)
	v := fib.Match(rq.interest.Name)
	if v == nil {
		return
	}
	key := exact.Key(fmt.Sprintf("%s/%x", rq.interest.Name, rq.interest.Nonce))
	forwarded.Update(key, func(fw interface{}) interface{} {
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
				forwarded.Remove(key)
			}()
		} else {
			rq.sender.log("loop detected", key)
		}
		return struct{}{}
	})
}
