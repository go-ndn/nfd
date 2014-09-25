package main

import (
	"fmt"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
)

type Face struct {
	*ndn.Face
	fib        *lpm.Matcher
	closed     chan<- *Face // unregister current face
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
	from     chan<- *ndn.Data // direct to original face
	to       <-chan *ndn.Data // save request after sending interest
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
			recv = recvFirst.to
		}

		// send data to requesting face
		var retFirst *ndn.Data
		var ret chan<- *ndn.Data
		if len(retPending) > 0 {
			retFirst = retPending[0].data
			ret = retPending[0].from
		}

		var closed chan<- *Face
		if len(sendPending) == 0 && len(recvPending) == 0 && len(retPending) == 0 && this.interestIn == nil {
			closed = this.closed
		}
		select {
		case i, ok := <-this.interestIn:
			this.log("interest in")
			if !ok {
				// this face will not accept new interest
				this.interestIn = nil
				this.log("face idle")
				continue
			}
			c := new(ndn.ControlInterest)
			err := ndn.Copy(i, c)
			if err == nil {
				// do not forward command to other faces
				d := &ndn.Data{
					Name: i.Name,
				}
				d.Content, err = ndn.Marshal(this.handleCommand(&c.Name), 101)
				if err != nil {
					continue
				}
				this.log("control response returned", d.Name)
				this.SendData(d)
				continue
			}
			// check for loop
			id := i.Name.String() + string(i.Nonce)
			if Forwarded[id] {
				continue
			}
			Forwarded[id] = true
			sendPending = append(sendPending, &bcast{
				interest: i,
				from:     this.dataOut,
			})
		case send <- sendFirst:
			sendPending = sendPending[1:]
		case b := <-this.bcastRecv:
			// interest name is the longest prefix of fib name
			if this.fib.RMatch(b.interest.Name) == nil {
				continue
			}
			this.log("interest forwarded", b.interest.Name)
			var err error
			b.to, err = this.SendInterest(b.interest)
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

func (this *Face) handleCommand(c *ndn.Command) (resp *ndn.ControlResponse) {
	service := c.Module + "." + c.Command
	this.log("_", service)
	if VerifyKey.Verify(c, c.SignatureValue.SignatureValue) != nil {
		resp = RespNotAuthorized
		return
	}
	resp = RespOK
	params := c.Parameters.Parameters
	switch service {
	case "fib.add-nexthop":
		if params.Cost == 0 {
			resp = RespIncorrectParams
			return
		}
		this.fib.Add(params.Name, params.Cost)
	case "fib.remove-nexthop":
		this.fib.Remove(params.Name)
	default:
		resp = RespNotSupported
	}
	return
}
