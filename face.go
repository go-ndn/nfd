package main

import (
	"fmt"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
)

type Face struct {
	*ndn.Face
	Closed chan *Face
	Bcast  chan *InterestBcast
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
	Sender   *Face
	Interest *ndn.Interest
}

func (this *Face) Listen() {
	defer func() {
		this.Close()
		this.Closed <- this
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
				d.Content, err = this.InternalDispatch(&c.Name)
				if err != nil {
					continue
				}
				this.log("control response returned", d.Name)
				this.SendData(d)
				continue
			}
			this.Bcast <- &InterestBcast{
				Interest: i,
				Sender:   this,
			}
		}
	}
}

func (this *Face) InternalDispatch(c *ndn.Command) (b []byte, err error) {
	service := c.Module + "." + c.Command
	this.log("_", service)
	params := c.Parameters.Parameters
	resp := RespOK
	// todo: authenticate
	switch service {
	case "fib.add-nexthop":
		this.Fib.Add(newLPMKey(params.Name), 0)
	case "fib.remove-nexthop":
		this.Fib.Remove(newLPMKey(params.Name))
	default:
		resp = RespNotSupported
	}
	b, err = ndn.Marshal(resp, 101)
	return
}
