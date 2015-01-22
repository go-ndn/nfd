package main

import (
	"fmt"

	"github.com/go-ndn/ndn"
)

type Face struct {
	*ndn.Face
	reqRecv      chan *req            // recv req from core
	interestRecv <-chan *ndn.Interest // recv interest from remote

	registered map[string]bool // true if prefix is registered directly
	id         string          // id != "" if face runs routing protocol
	cost       uint64          // cost != 0 if core initiates connection
}

func (this *Face) log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[%s] %s", this.RemoteAddr(), fmt.Sprintln(i...))
}

type req struct {
	sender   *Face
	interest *ndn.Interest
	resp     chan (<-chan *ndn.Data) // recv resp from core
}

func (this *Face) Run() {
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

		if this.interestRecv == nil {
			// when face is closing, it will not send to other faces
			faceClose = FaceClose
		} else if len(sendPending) > 0 {
			sendFirst = sendPending[0]
			send = ReqSend
		}
		select {
		case i, ok := <-this.interestRecv:
			if !ok {
				// this face will not accept new interest
				this.interestRecv = nil
				this.log("face idle")
				continue
			}
			this.log("recv interest", i.Name)
			sendPending = append(sendPending, &req{
				sender:   this,
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
			this.log("send data", d.Name)
			this.SendData(d)
		case b := <-this.reqRecv:
			if this.interestRecv != nil {
				ch, err := this.SendInterest(b.interest)
				if err == nil {
					sender := "core"
					if b.sender != nil {
						sender = b.sender.RemoteAddr().String()
					}
					this.log("forward", b.interest.Name, "from", sender)
					b.resp <- ch
				}
			}
			close(b.resp)
		case faceClose <- this:
			this.Close()
			close(recvDone)
			return
		}
	}
}
