package main

import (
	"crypto/sha256"
	"fmt"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
	"github.com/taylorchu/tlv"
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

func newLPMKey(n ndn.Name) (cs []lpm.Component) {
	for _, c := range n.Components {
		cs = append(cs, lpm.Component(c))
	}
	return
}

func newSha256(v interface{}) (digest []byte, err error) {
	h := sha256.New()
	err = tlv.Data(h, v)
	if err != nil {
		return
	}
	digest = h.Sum(nil)
	return
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
			if this.fib.Match(newLPMKey(b.interest.Name)) == nil {
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
			e := this.fib.Match(newLPMKey(b.name))
			if e != nil && e.(uint64) < b.cost {
				continue
			}
			go func() {
				var err error
				if b.cost == 0 {
					err = this.RemoveNextHop(b.name.String())
				} else {
					err = this.AddNextHop(b.name.String(), b.cost)
				}
				if err != nil {
					this.log(err)
				}
			}()
		case d := <-this.dataOut:
			this.log("data returned", d.Name)
			this.SendData(d)
		}
	}
}

func (this *Face) handleCommand(c *ndn.Command) (resp *ndn.ControlResponse) {
	service := c.Module + "." + c.Command
	this.log("_", service)
	digest, err := newSha256(c)
	if err != nil || VerifyKey.Verify(digest, c.SignatureValue.SignatureValue) != nil {
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
		this.fib.Add(newLPMKey(params.Name), params.Cost)
		this.bcastFibSend <- &fibBcast{
			name: params.Name,
			cost: params.Cost + 1,
		}
	case "fib.remove-nexthop":
		this.fib.Remove(newLPMKey(params.Name))
		this.bcastFibSend <- &fibBcast{
			name: params.Name,
		}
	default:
		resp = RespNotSupported
	}
	return
}
