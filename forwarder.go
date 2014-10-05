package main

import (
	"github.com/taylorchu/exact"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
	"log"
	"net"
	"strings"
	"time"
)

type Forwarder struct {
	id         string
	forwarded  *exact.Matcher
	verifyKey  ndn.Key
	faceCreate chan *connInfo
	face       map[*Face]bool

	rib        map[string]*ndn.LSA
	fib        *lpm.Matcher
	ribUpdated bool
}

type connInfo struct {
	conn net.Conn
	cost uint64
}

func (this *Forwarder) Run() {
	reqSend := make(chan *req)
	closed := make(chan *Face)
	floodTimer := time.Tick(FloodTimer)
	expireTimer := time.Tick(ExpireTimer)
	var nextHop <-chan map[string]ndn.Neighbor
	for {
		fibUpdate := make(chan interface{})
		if this.ribUpdated {
			close(fibUpdate)
		}
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
		case <-fibUpdate:
			this.ribUpdated = false
			log.Println("recompute fib")
			// copy rib
			var state []*ndn.LSA
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
			log.Println("finish fib update")
			this.fib = this.updateFib(b)
		case <-floodTimer:
			log.Println("flood lsa")
			this.flood(this.id, nil)
		case <-expireTimer:
			log.Println("remove expired lsa")
			this.removeExpiredLSA()
		case f := <-closed:
			delete(this.face, f)
			this.fib.Visit(func(chs interface{}) interface{} {
				m := chs.(map[chan<- *req]bool)
				delete(m, f.reqRecv)
				if len(m) == 0 {
					return nil
				}
				return m
			})
			// prefix or neighbor id removed
			this.updateLSA(this.localLSA())
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
			}
		}
		go func() {
			time.Sleep(ForwardTimer)
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
	b.resp <- ch
}

func (this *Forwarder) handleCommand(c *ndn.Command, f *Face) (resp *ndn.ControlResponse) {
	if this.verifyKey.Verify(c, c.SignatureValue.SignatureValue) != nil {
		resp = RespNotAuthorized
		return
	}
	resp = RespOK
	params := c.Parameters.Parameters
	switch c.Module + "/" + c.Command {
	case "fib/add-nexthop":
		f.registered[params.Name.String()] = true
		// name added
		this.updateLSA(this.localLSA())
	case "fib/remove-nexthop":
		delete(f.registered, params.Name.String())
		// name removed
		this.updateLSA(this.localLSA())
	case "lsa/flood":
		if !this.canFlood(&params.LSA) {
			return
		}
		f.log("lsa", params.LSA.Id, params.Uri)
		this.updateLSA(&params.LSA)
		if f.id != params.Uri {
			f.id = params.Uri
			if f.cost != 0 {
				// neighbor id learned
				this.updateLSA(this.localLSA())
			}
		}
		this.flood(params.LSA.Id, f)
	default:
		resp = RespNotSupported
	}
	return
}
