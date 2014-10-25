package main

import (
	"fmt"
	"github.com/taylorchu/ndn"
	"sync"
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

func merge(done <-chan interface{}, cs ...<-chan *ndn.Data) <-chan *ndn.Data {
	var wg sync.WaitGroup
	out := make(chan *ndn.Data)

	// Start an output goroutine for each input channel in cs.  output
	// copies values from c to out until c is closed, then calls wg.Done.
	output := func(c <-chan *ndn.Data) {
		defer wg.Done()
		var sendPending []*ndn.Data
		for {
			var send chan<- *ndn.Data
			var sendFirst *ndn.Data
			if len(sendPending) > 0 {
				send = out
				sendFirst = sendPending[0]
			} else if c == nil {
				return
			}
			select {
			case d, ok := <-c:
				if !ok {
					c = nil
					continue
				}
				sendPending = append(sendPending, d)
			case send <- sendFirst:
				sendPending = sendPending[1:]
			case <-done:
				return
			}
		}
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	// Start a goroutine to close out once all the output goroutines are
	// done.  This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func (this *Face) Run() {
	// send req with queue
	var sendPending []*req

	// listen to pending receive at the same time
	var recv <-chan *ndn.Data
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
			this.log("interest in", i.Name)
			sendPending = append(sendPending, &req{
				interest: i,
				sender:   this,
				resp:     make(chan (<-chan *ndn.Data)),
			})
		case send <- sendFirst:
			sendPending = sendPending[1:]
			var recvPending []<-chan *ndn.Data
			if recv != nil {
				recvPending = append(recvPending, recv)
			}
			for ch := range sendFirst.resp {
				recvPending = append(recvPending, ch)
			}
			// merge and listen to more channels
			recv = merge(recvDone, recvPending...)
		case d, ok := <-recv:
			if !ok {
				recv = nil
				continue
			}
			// data is shared by other faces, so making copy is required to avoid data race
			copy := *d
			this.log("data returned", copy.Name)
			this.SendData(&copy)
		case b := <-this.reqRecv:
			if this.interestIn == nil {
				// listening is required even if idle, so forwarder will not block
				close(b.resp)
				continue
			}
			// interest is shared by other faces, so making copy is required to avoid data race
			copy := *b.interest
			ch, err := this.SendInterest(&copy)
			sender := "core"
			if b.sender != nil {
				sender = b.sender.RemoteAddr().String()
			}
			if err == nil {
				this.log("interest forwarded", copy.Name, sender)
				b.resp <- ch
			}
			close(b.resp)
		case closed <- this:
			this.Close()
			close(recvDone)
			close(this.reqRecv)
			return
		}
	}
}
