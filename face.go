package main

import (
	"fmt"
	"github.com/taylorchu/ndn"
)

type Face struct {
	*ndn.Face
	nextHops   map[string]bool // names in fib
	closed     chan<- *Face    // unregister current face
	bcastSend  chan<- *bcast
	bcastRecv  chan *bcast
	interestIn <-chan *ndn.Interest
	dataOut    chan *ndn.Data // data that will be written to current face
}

func (this *Face) log(i ...interface{}) {
	fmt.Printf("[%s] ", this.RemoteAddr())
	fmt.Println(i...)
}

type bcast struct {
	sender   *Face            // original face
	pending  <-chan *ndn.Data // save request after sending interest
	data     *ndn.Data        // fetch data and save it
	interest *ndn.Interest
}

func (this *Face) Listen() {
	var sendPending []*bcast
	var recvPending []*bcast
	var retPending []*bcast
	for {
		// send interest to other face
		var send chan<- *bcast
		var sendFirst *bcast
		if len(sendPending) > 0 {
			sendFirst = sendPending[0]
			send = this.bcastSend
		}

		// fetch data from this face
		var recvFirst *bcast
		var recv <-chan *ndn.Data
		if len(recvPending) > 0 {
			recvFirst = recvPending[0]
			recv = recvFirst.pending
		}

		// send data to requesting face
		var retFirst *ndn.Data
		var ret chan<- *ndn.Data
		if len(retPending) > 0 {
			retFirst = retPending[0].data
			ret = retPending[0].sender.dataOut
		}

		var closed chan<- *Face
		if len(sendPending) == 0 && len(recvPending) == 0 && len(retPending) == 0 && this.interestIn == nil {
			closed = this.closed
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
			id := i.Name.String() + string(i.Nonce)
			if Forwarded[id] {
				continue
			}
			this.log("interest in", i.Name)
			Forwarded[id] = true

			sendPending = append(sendPending, &bcast{
				interest: i,
				sender:   this,
			})
		case send <- sendFirst:
			sendPending = sendPending[1:]
		case b := <-this.bcastRecv:
			this.log("interest forwarded", b.interest.Name)
			var err error
			b.pending, err = this.SendInterest(b.interest)
			if err != nil {
				this.log(err)
				continue
			}
			recvPending = append(recvPending, b)
		case recvFirst.data = <-recv:
			recvPending = recvPending[1:]
			if recvFirst.data != nil {
				retPending = append(retPending, recvFirst)
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
