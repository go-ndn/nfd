package main

import (
	"fmt"
	"github.com/taylorchu/exact"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
	"net"
	"strings"
	"time"
)

type Forwarder struct {
	id         string
	forwarded  *exact.Matcher
	faceCreate chan *connInfo
	face       map[*Face]bool

	rib        map[string]*ndn.LSA
	fib        *lpm.Matcher
	ribUpdated bool

	verifyKey ndn.Key
	timestamp uint64
}

type connInfo struct {
	conn net.Conn
	cost uint64
}

func log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[core] %s", fmt.Sprintln(i...))
}

func (this *Forwarder) Run() {
	reqSend := make(chan *req)
	closed := make(chan *Face)
	var (
		floodTimer, expireTimer, fibTimer <-chan time.Time
		nextHop                           <-chan map[string]ndn.Neighbor
	)
	if !*dummy {
		floodTimer = time.Tick(FloodTimer)
		expireTimer = time.Tick(ExpireTimer)
		fibTimer = time.Tick(FibTimer)
	}
	for {
		select {
		case info := <-this.faceCreate:
			ch := make(chan *ndn.Interest)
			f := &Face{
				Face:       ndn.NewFace(info.conn, ch),
				reqSend:    reqSend,
				reqRecv:    make(chan *req),
				interestIn: ch,
				closed:     closed,
				registered: make(map[string]bool),
				cost:       info.cost,
			}
			this.face[f] = true
			f.log("face created")
			go f.Run()
		case b := <-reqSend:
			this.handleReq(b)
		case <-fibTimer:
			if !this.ribUpdated {
				continue
			}
			this.ribUpdated = false
			log("recompute fib")
			// copy rib
			state := []*ndn.LSA{this.localLSA()}
			for _, v := range this.rib {
				state = append(state, v)
			}
			ch := make(chan map[string]ndn.Neighbor, 1)
			go func() {
				ch <- computeNextHop(this.id, state)
				close(ch)
			}()
			nextHop = ch
		case b := <-nextHop:
			nextHop = nil
			log("finish fib update")
			this.updateFib(b)
		case <-floodTimer:
			log("flood lsa")
			this.flood(this.localLSA(), nil)
		case <-expireTimer:
			log("remove expired lsa")
			this.removeExpiredLSA()
		case f := <-closed:
			delete(this.face, f)
			for name := range f.registered {
				this.removeNextHop(name, f)
			}
			f.log("face removed")
		}
	}
}

func (this *Forwarder) handleReq(b *req) {
	defer close(b.resp)
	if strings.HasPrefix(b.interest.Name.String(), "/localhost/nfd/") {
		this.handleLocal(b)
		return
	}
	chs := this.fib.Match(b.interest.Name)
	if chs == nil {
		return
	}
	k := exact.Key(b.interest.Name.String() + string(b.interest.Nonce))
	this.forwarded.Update(k, func(v interface{}) interface{} {
		if v != nil {
			// loop, ignore req
			return v
		}
		for ch := range chs.(map[chan<- *req]bool) {
			resp := make(chan (<-chan *ndn.Data))
			ch <- &req{
				interest: b.interest,
				sender:   b.sender,
				resp:     resp,
			}
			r, ok := <-resp
			if ok {
				b.resp <- r
				break
			}
		}
		go func() {
			time.Sleep(LoopTimer)
			this.forwarded.Remove(k)
		}()
		return true
	})
}

func (this *Forwarder) handleLocal(b *req) {
	d := &ndn.Data{
		Name: b.interest.Name,
	}
	c := new(ndn.ControlInterest)
	err := ndn.Copy(b.interest, c)
	if err != nil {
		return
	}
	d.Content, err = ndn.Marshal(this.handleCommand(&c.Name, b.sender), 101)
	if err != nil {
		return
	}
	ch := make(chan *ndn.Data, 1)
	ch <- d
	close(ch)
	b.resp <- ch
}

func (this *Forwarder) handleCommand(c *ndn.Command, f *Face) (resp *ndn.ControlResponse) {
	if c.Timestamp <= this.timestamp || this.verifyKey.Verify(c, c.SignatureValue.SignatureValue) != nil {
		resp = RespNotAuthorized
		return
	}
	this.timestamp = c.Timestamp
	resp = RespOK
	params := c.Parameters.Parameters
	switch c.Module + "/" + c.Command {
	case "rib/register":
		this.addNextHop(params.Name.String(), f, true)
		if *dummy {
			this.transferCommand(c)
		}
	case "rib/unregister":
		this.removeNextHop(params.Name.String(), f)
		if *dummy {
			this.transferCommand(c)
		}
	case "lsa/flood":
		if *dummy || !this.canFlood(params.LSA) {
			return
		}
		f.log("lsa", params.LSA.Id, params.Uri)
		this.rib[params.LSA.Id] = params.LSA
		this.ribUpdated = true
		f.id = params.Uri
		this.flood(params.LSA, f)
	default:
		resp = RespNotSupported
	}
	return
}
