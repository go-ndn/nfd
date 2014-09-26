package main

import (
	"fmt"
	"github.com/taylorchu/exact"
	"github.com/taylorchu/ndn"
)

type Face struct {
	*ndn.Face
	fibNames   map[string]bool // names in fib
	closed     chan<- *Face    // unregister current face
	bcastSend  chan<- *bcast
	bcastRecv  chan *bcast
	interestIn <-chan *ndn.Interest
	dataOut    chan *ndn.Data // data that will be written to current face
}

func (this *Face) log(i ...interface{}) {
	fmt.Printf("[%s] %s", this.RemoteAddr(), fmt.Sprintln(i...))
}

type bcast struct {
	sender   *Face            // original face
	pending  <-chan *ndn.Data // save request after sending interest (channel will return a shared pointer)
	data     ndn.Data         // data fetched from pending
	interest ndn.Interest     // copy of original interest
}

func (this *Face) Listen() {
	var sendPending []*bcast
	var recvPending []*bcast
	var retPending []*bcast
	for {
		// send interest to other face
		var send chan<- *bcast
		var sendFirst *bcast

		// fetch data from this face
		var recvFirst *bcast
		var recv <-chan *ndn.Data

		// send data to requesting face
		var retFirst *ndn.Data
		var ret chan<- *ndn.Data

		// shutdown
		var closed chan<- *Face

		if this.interestIn == nil {
			closed = this.closed
		} else {
			if len(sendPending) > 0 {
				sendFirst = sendPending[0]
				send = this.bcastSend
			}
			if len(recvPending) > 0 {
				recvFirst = recvPending[0]
				recv = recvFirst.pending
			}
			if len(retPending) > 0 {
				retFirst = &retPending[0].data
				ret = retPending[0].sender.dataOut
			}
		}

		select {
		case i, ok := <-this.interestIn:
			if !ok {
				// this face will not accept new interest
				this.interestIn = nil
				this.log("face idle")
				continue
			}
			// check for loop
			Forwarded.Update(exact.Key(i.Name.String()+string(i.Nonce)), func(v interface{}) interface{} {
				if v != nil {
					return v
				}
				this.log("interest in", i.Name)
				sendPending = append(sendPending, &bcast{
					interest: *i,
					sender:   this,
				})
				return true
			})
		case send <- sendFirst:
			sendPending = sendPending[1:]
		case b := <-this.bcastRecv:
			var err error
			b.pending, err = this.SendInterest(&b.interest)
			if err != nil {
				this.log(err)
				continue
			}
			this.log("interest forwarded", b.interest.Name, b.sender.RemoteAddr())
			recvPending = append(recvPending, b)
		case d, ok := <-recv:
			recvPending = recvPending[1:]
			if ok {
				// a copy must be created because this fetched data might also be given to other fetching faces
				recvFirst.data = *d
				retPending = append(retPending, recvFirst)
			} else {
				this.log("no data", recvFirst.sender.RemoteAddr())
			}
		case ret <- retFirst:
			retPending = retPending[1:]
		case d := <-this.dataOut:
			this.log("data returned", d.Name)
			this.SendData(d)
		case closed <- this:
			this.Close()
			return
		}
	}
}
