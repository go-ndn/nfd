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
	FIB       = lpm.New()
)

func log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[core] %s\n", fmt.Sprint(i...))
}

func handleLocal() {
	for _, route := range localRoute {
		reqRecv := make(chan *req)
		AddNextHop(lpm.Key(route.URL), reqRecv)
		go func(route Route) {
			for {
				rq := <-reqRecv
				var (
					v interface{}
					t uint64
				)
				if route.HandleCommand != nil {
					// command
					t = 101
					cmd := new(ndn.Command)
					ndn.Copy(&rq.interest.Name, cmd)
					if cmd.Timestamp <= Timestamp || VerifyKey.Verify(cmd, cmd.SignatureValue.SignatureValue) != nil {
						v = RespNotAuthorized
						goto REQ_DONE
					}
					Timestamp = cmd.Timestamp
					params := &cmd.Parameters.Parameters

					var face *Face
					if params.FaceID == 0 {
						face = rq.sender
					} else {
						face = (*Face)(unsafe.Pointer(uintptr(params.FaceID)))
						if _, ok := Faces[face]; !ok {
							v = RespIncorrectParams
							goto REQ_DONE
						}
					}

					v = route.HandleCommand(params, face)
				} else {
					// dataset
					t = 128
					v = route.HandleDataset()
				}

			REQ_DONE:
				d := &ndn.Data{Name: rq.interest.Name}
				d.Content, _ = ndn.Marshal(v, t)
				ch := make(chan *ndn.Data, 1)
				ch <- d
				close(ch)
				rq.resp <- ch
				close(rq.resp)
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
			face := &Face{
				Face:         ndn.NewFace(conn, ch),
				reqRecv:      make(chan *req),
				interestRecv: ch,
				route:        make(map[string]ndn.Route),
			}
			Faces[face] = struct{}{}
			face.log("face created")
			go face.Run()
		case rq := <-ReqSend:
			HandleReq(rq)
		case face := <-FaceClose:
			delete(Faces, face)
			for name := range face.route {
				RemoveNextHop(ndn.NewName(name), face.reqRecv)
			}
			face.log("face removed")
		case <-quit:
			log("goodbye")
			return
		}
	}
}

func AddNextHop(name fmt.Stringer, ch chan<- *req) {
	FIB.Update(name, func(v interface{}) interface{} {
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

func RemoveNextHop(name fmt.Stringer, ch chan<- *req) {
	FIB.Update(name, func(v interface{}) interface{} {
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

func HandleReq(rq *req) {
	defer close(rq.resp)
	v := FIB.Match(rq.interest.Name)
	if v == nil {
		return
	}
	key := exact.Key(fmt.Sprintf("%s/%x", rq.interest.Name, rq.interest.Nonce))
	Forwarded.Update(key, func(fw interface{}) interface{} {
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
				time.Sleep(LoopDetectIntv)
				Forwarded.Remove(key)
			}()
		} else {
			rq.sender.log("loop detected", key)
		}
		return struct{}{}
	})
}
