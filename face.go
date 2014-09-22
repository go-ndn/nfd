package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
)

func prepare(n ndn.Name) (cs []lpm.Component) {
	for _, c := range n.Components {
		cs = append(cs, lpm.Component(c))
	}
	return
}

func (this *Face) debug(i ...interface{}) {
	fmt.Printf("[%s] ", this.c.RemoteAddr())
	fmt.Println(i...)
}

func (this *Face) Incoming() {
	defer func() {
		this.c.Close()
		this.closed = true
	}()
	for {
		switch p := (<-this.in).(type) {
		case *ndn.Interest:
			key := prepare(p.Name)
			e := this.fib.RMatch(key)
			if e == nil {
				// prefix not announced
				continue
			}
			this.debug("interest forwarded", p.Name)
			p.WriteTo(this.c)
		case *ndn.Data:
			key := prepare(p.Name)
			e := this.pit.Match(key)
			if e == nil {
				// not in pit
				this.debug("data dropped; not in pit", p.Name)
				continue
			}
			this.debug("data forwarded", p.Name)
			this.pit.Update(key, func(interface{}) interface{} { return nil })
			ContentStore.Set(key, p)
			this.debug("cache insert", ContentStore.List())
			p.WriteTo(this.c)
		default:
			return
		}
	}
}

func (this *Face) Outgoing() {
	defer func() {
		this.c.Close()
		this.closed = true
	}()
	r := bufio.NewReader(this.c)
	for {
		i := new(ndn.Interest)
		err := i.ReadFrom(r)
		if err == nil {
			// internal service
			buf := new(bytes.Buffer)
			i.WriteTo(buf)
			c := new(ndn.ControlPacket)
			err = c.ReadFrom(bufio.NewReader(buf))
			if err == nil {
				d := &ndn.Data{
					Name: i.Name,
				}
				d.Content, err = this.InternalDispatch(&c.Name)
				if err != nil {
					continue
				}
				d.WriteTo(this.c)
				continue
			}
			key := prepare(i.Name)
			e := ContentStore.RMatch(key)
			if e != nil {
				this.debug("data found in content store", i.Name)
				e.(*ndn.Data).WriteTo(this.c)
				// found in cache
				continue
			}
			e = this.pit.RMatch(key)
			if e != nil {
				// already in pit
				this.debug("interest dropped; already in pit", i.Name)
				continue
			}
			this.pit.Set(key, true)
			this.debug("interest for other faces", i.Name)
			this.out <- i
			continue
		}
		d := new(ndn.Data)
		err = d.ReadFrom(r)
		if err == nil {
			this.debug("data for other faces", d.Name)
			this.out <- d
			continue
		}
		return
	}
}

func (this *Face) InternalDispatch(c *ndn.Command) (b []byte, err error) {
	service := c.Module + "." + c.Command
	this.debug("_", service)
	params := c.Parameters.Parameters
	resp := RespOK
	// todo: authenticate
	if c.SignatureInfo.SignatureInfo.SignatureType == ndn.SignatureTypeDigestSha256 {
		resp = RespNotAuthorized
	} else {
		switch service {
		case "faces.create":
		case "fib.add-nexthop":
			this.fib.Set(prepare(params.Name), true)
		case "fib.remove-nexthop":
			this.fib.Update(prepare(params.Name), func(interface{}) interface{} { return nil })
		default:
			resp = RespNotSupported
		}
	}
	b, err = ndn.Marshal(resp, 101)
	return
}
