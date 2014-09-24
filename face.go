package main

import (
	"fmt"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
)

type Face struct {
	*ndn.Face
	fib          *lpm.Matcher
	closed       chan *Face
	bcastFibSend chan *fibBcast
	bcastFibRecv chan *fibBcast
	bcastSend    chan *interestBcast
	bcastRecv    chan *interestBcast
	dataOut      chan *ndn.Data
}

func (this *Face) log(i ...interface{}) {
	fmt.Printf("[%s] ", this.RemoteAddr())
	fmt.Println(i...)
}

type fibBcast struct {
	name ndn.Name
	cost uint64
}

type interestBcast struct {
	sender   chan *ndn.Data
	interest *ndn.Interest
}

func (this *Face) Listen() {
	defer func() {
		this.Close()
		this.closed <- this
	}()
	for {
		select {
		case i, ok := <-this.InterestIn:
			if !ok {
				return
			}
			c := new(ndn.ControlInterest)
			err := ndn.Copy(i, c)
			if err == nil {
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
			this.bcastSend <- &interestBcast{
				interest: i,
				sender:   this.dataOut,
			}
		case b := <-this.bcastRecv:
			if this.fib.Match(b.interest.Name) == nil {
				continue
			}
			this.log("interest forwarded", b.interest.Name)
			ch, err := this.SendInterest(b.interest)
			if err != nil {
				this.log(err)
				continue
			}
			go func() {
				d, ok := <-ch
				if !ok {
					return
				}
				b.sender <- d
			}()
		case b := <-this.bcastFibRecv:
			e := this.fib.Match(b.name)
			if e != nil && e.(uint64) < b.cost {
				continue
			}
			if b.cost == 0 {
				if nil == this.RemoveNextHop(b.name.String()) {
					this.log("remove next hop", b.name)
				}
			} else {
				if nil == this.AddNextHop(b.name.String(), b.cost) {
					this.log("add next hop", b.name, b.cost)
				}
			}
		case d := <-this.dataOut:
			this.log("data returned", d.Name)
			this.SendData(d)
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
		this.bcastFibSend <- &fibBcast{
			name: params.Name,
			cost: params.Cost + 1,
		}
	case "fib.remove-nexthop":
		this.fib.Remove(params.Name)
		this.bcastFibSend <- &fibBcast{
			name: params.Name,
		}
	default:
		resp = RespNotSupported
	}
	return
}
