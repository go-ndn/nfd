package main

import (
	"fmt"

	"github.com/go-ndn/ndn"
)

type Face struct {
	*ndn.Face
	reqRecv      chan *req            // recv req from core
	interestRecv <-chan *ndn.Interest // recv interest from remote

	route map[string]ndn.Route
}

func (f *Face) log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[%s] %s", f.RemoteAddr(), fmt.Sprintln(i...))
}

type req struct {
	sender   *Face
	interest *ndn.Interest
	resp     chan (<-chan *ndn.Data) // recv resp from core
}

func (f *Face) Run() {
	// send req with queue
	var sendPending []*req

	// listen to pending receive at the same time
	recv := make(chan *ndn.Data)
	recvDone := make(chan struct{})

	for {
		// send interest to other face
		var send chan<- *req
		var sendFirst *req

		// shutdown
		var faceClose chan<- *Face

		if f.interestRecv == nil {
			// when face is closing, it will not send to other faces
			faceClose = FaceClose
		} else if len(sendPending) > 0 {
			sendFirst = sendPending[0]
			send = ReqSend
		}
		select {
		case i, ok := <-f.interestRecv:
			if !ok {
				// this face will not accept new interest
				f.interestRecv = nil
				f.log("face idle")
				continue
			}
			f.log("recv interest", i.Name)
			sendPending = append(sendPending, &req{
				sender:   f,
				interest: i,
				resp:     make(chan (<-chan *ndn.Data)),
			})
		case send <- sendFirst:
			sendPending = sendPending[1:]
			for ch := range sendFirst.resp {
				go func(ch <-chan *ndn.Data) {
					select {
					case d, ok := <-ch:
						if !ok {
							return
						}
						select {
						case recv <- d:
						case <-recvDone:
						}
					case <-recvDone:
					}
				}(ch)
			}
		case d := <-recv:
			f.log("send data", d.Name)
			f.SendData(d)
		case rq := <-f.reqRecv:
			if f.interestRecv != nil {
				ch, err := f.SendInterest(rq.interest)
				if err == nil {
					sender := "core"
					if rq.sender != nil {
						sender = rq.sender.RemoteAddr().String()
					}
					f.log("forward", rq.interest.Name, "from", sender)
					rq.resp <- ch
				}
			}
			close(rq.resp)
		case faceClose <- f:
			f.Close()
			close(recvDone)
			return
		}
	}
}
