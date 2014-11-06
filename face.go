package main

import (
	"fmt"
	"github.com/taylorchu/ndn"
)

type Face struct {
	*ndn.Face
	closed     chan<- *Face // remove face
	reqSend    chan<- *req
	reqRecv    chan *req
	interestIn <-chan *ndn.Interest

	registered map[string]bool
	id         string
	cost       uint64
}

func (this *Face) log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[%s] %s", this.RemoteAddr(), fmt.Sprintln(i...))
}

type req struct {
	sender   *Face         // original face
	interest *ndn.Interest // interest from original face
	resp     chan (<-chan *ndn.Data)
}

func (this *Face) Run() {
	// send req with queue
	var sendPending []*req

	// listen to pending receive at the same time
	recv := make(chan *ndn.Data)
	recvDone := make(chan interface{})
	for {
		// send interest to other face
		var send chan<- *req
		var sendFirst *req

		// shutdown
		var closed chan<- *Face

		if this.interestIn == nil {
			// when face is closing, it will not send to other faces
			closed = this.closed
		} else if len(sendPending) > 0 {
			sendFirst = sendPending[0]
			send = this.reqSend
		}
		select {
		case i, ok := <-this.interestIn:
			if !ok {
				// this face will not accept new interest
				this.interestIn = nil
				this.log("face idle")
				continue
			}
			this.log("recv interest", i.Name)
			sendPending = append(sendPending, &req{
				interest: i,
				sender:   this,
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
			if this.interestIn == nil {
				// listening is required even if idle, so forwarder will not block
				close(b.resp)
				continue
			}
			ch, err := this.SendInterest(b.interest)
			sender := "core"
			if b.sender != nil {
				sender = b.sender.RemoteAddr().String()
			}
			if err == nil {
				this.log("forward", b.interest.Name, "from", sender)
				b.resp <- ch
			}
			close(b.resp)
		case closed <- this:
			this.Close()
			close(recvDone)
			return
		}
	}
}
