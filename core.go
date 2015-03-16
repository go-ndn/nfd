package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ndn/exact"
	"github.com/go-ndn/lpm"
	"github.com/go-ndn/ndn"
)

var (
	Faces = make(map[*Face]struct{})

	FaceCreate = make(chan net.Conn)
	ReqSend    = make(chan *req)
	FaceClose  = make(chan *Face)

	Forwarded = exact.New()
	Fib       = lpm.New()
)

func log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[core] %s", fmt.Sprintln(i...))
}

func handleLocal() {
	for _, route := range localRoute {
		reqRecv := make(chan *req)
		AddNextHop(lpm.Key(route.URL), reqRecv)
		go func(route Route) {
			for {
				b := <-reqRecv
				var (
					v interface{}
					t uint64
				)
				if route.HandleCommand != nil {
					// command
					t = 101
					c := new(ndn.Command)
					ndn.Copy(&b.interest.Name, c)
					if c.Timestamp <= Timestamp || VerifyKey.Verify(c, c.SignatureValue.SignatureValue) != nil {
						v = RespNotAuthorized
						goto REQ_DONE
					}
					Timestamp = c.Timestamp
					params := &c.Parameters.Parameters

					var f *Face
					if params.FaceId == 0 {
						f = b.sender
					} else {
						f = (*Face)(unsafe.Pointer(uintptr(params.FaceId)))
						if _, ok := Faces[f]; !ok {
							v = RespIncorrectParams
							goto REQ_DONE
						}
					}

					v = route.HandleCommand(params, f)
				} else {
					// dataset
					t = 80
					v = route.HandleDataset()
				}

			REQ_DONE:
				if v != nil {
					d := &ndn.Data{Name: b.interest.Name}
					d.Content, _ = ndn.Marshal(v, t)
					ch := make(chan *ndn.Data, 1)
					ch <- d
					close(ch)
					b.resp <- ch
				}
				close(b.resp)
			}
		}(route)
	}
}

func Run() {
	log("start")

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case conn := <-FaceCreate:
			ch := make(chan *ndn.Interest)
			f := &Face{
				Face:         ndn.NewFace(conn, ch),
				reqRecv:      make(chan *req),
				interestRecv: ch,
				route:        make(map[string]ndn.Route),
			}
			Faces[f] = struct{}{}
			f.log("face created")
			go f.Run()
		case b := <-ReqSend:
			HandleReq(b)
		case f := <-FaceClose:
			delete(Faces, f)
			for name := range f.route {
				RemoveNextHop(ndn.NewName(name), f.reqRecv)
			}
			f.log("face removed")
		case <-quit:
			log("goodbye")
			return
		}
	}
}

func AddNextHop(name fmt.Stringer, ch chan<- *req) {
	Fib.Update(name, func(chs interface{}) interface{} {
		var m map[chan<- *req]struct{}
		if chs == nil {
			m = make(map[chan<- *req]struct{})
		} else {
			m = chs.(map[chan<- *req]struct{})
		}
		m[ch] = struct{}{}
		return m
	}, false)
}

func RemoveNextHop(name fmt.Stringer, ch chan<- *req) {
	Fib.Update(name, func(chs interface{}) interface{} {
		if chs == nil {
			return nil
		}
		m := chs.(map[chan<- *req]struct{})
		delete(m, ch)
		if len(m) == 0 {
			return nil
		}
		return m
	}, false)
}

func HandleReq(b *req) {
	defer close(b.resp)
	chs := Fib.Match(b.interest.Name)
	if chs == nil {
		return
	}
	k := exact.Key(fmt.Sprintf("%s/%x", b.interest.Name, b.interest.Nonce))
	Forwarded.Update(k, func(v interface{}) interface{} {
		if v != nil {
			b.sender.log("loop detected", k)
			return v
		}
		for ch := range chs.(map[chan<- *req]struct{}) {
			resp := make(chan (<-chan *ndn.Data))
			ch <- &req{
				interest: b.interest,
				sender:   b.sender,
				resp:     resp,
			}
			r, ok := <-resp
			if ok {
				b.resp <- r
				break
			}
		}
		go func() {
			time.Sleep(LoopDetectIntv)
			Forwarded.Remove(k)
		}()
		return struct{}{}
	})
}
