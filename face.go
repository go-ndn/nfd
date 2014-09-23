package main

import (
	"fmt"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
)

type Face struct {
	*ndn.Face
	fib       *lpm.Matcher
	closed    chan *Face
	bcastSend chan *InterestBcast
	bcastRecv chan *InterestBcast
	dataOut   chan *ndn.Data
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

type InterestBcast struct {
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
				d.Content, err = this.internalDispatch(&c.Name)
				if err != nil {
					continue
				}
				this.log("control response returned", d.Name)
				this.SendData(d)
				continue
			}
			this.bcastSend <- &InterestBcast{
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
		case d := <-this.dataOut:
			this.log("data returned", d.Name)
			this.SendData(d)
		}
	}
}

func (this *Face) internalDispatch(c *ndn.Command) (b []byte, err error) {
	service := c.Module + "." + c.Command
	this.log("_", service)
	params := c.Parameters.Parameters
	resp := RespOK
	// todo: authenticate
	switch service {
	case "fib.add-nexthop":
		this.fib.Add(newLPMKey(params.Name), params.Cost)
	case "fib.remove-nexthop":
		this.fib.Remove(newLPMKey(params.Name))
	default:
		resp = RespNotSupported
	}
	b, err = ndn.Marshal(resp, 101)
	return
}
