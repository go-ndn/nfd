package main

import (
	"strings"
	"time"

	"github.com/taylorchu/exact"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
)

type Forwarder struct {
	id         string
	faceCreate chan *connInfo
	face       map[*Face]bool

	forwarded *exact.Matcher
	fib       *lpm.Matcher

	rib        map[string]*ndn.LSA
	ribUpdated bool
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
			log("compute best route")
			nextHop = this.bestRoute()
		case b := <-nextHop:
			nextHop = nil
			log("finish fib update")
			this.updateFib(b)
		case <-floodTimer:
			log("flood lsa")
			this.flood(this.createLSA(), nil)
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
			// loop detected
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
	control := new(ndn.ControlInterest)
	err := ndn.Copy(b.interest, control)
	if err != nil {
		return
	}
	d := &ndn.Data{Name: b.interest.Name}
	d.Content, err = ndn.Marshal(this.handleCommand(&control.Name, b.sender), 101)
	if err != nil {
		return
	}
	ch := make(chan *ndn.Data, 1)
	ch <- d
	close(ch)
	b.resp <- ch
}

func (this *Forwarder) handleCommand(c *ndn.Command, f *Face) (resp *ndn.ControlResponse) {
	if c.Timestamp <= Timestamp || VerifyKey.Verify(c, c.SignatureValue.SignatureValue) != nil {
		resp = RespNotAuthorized
		return
	}
	Timestamp = c.Timestamp
	resp = RespOK
	params := c.Parameters.Parameters
	switch c.Module + "/" + c.Command {
	case "rib/register":
		this.addNextHop(params.Name.String(), f, true)
		if *dummy {
			this.forwardControl(c.Module, c.Command, &c.Parameters.Parameters, func(f *Face) bool { return f.cost != 0 })
		}
	case "rib/unregister":
		this.removeNextHop(params.Name.String(), f)
		if *dummy {
			this.forwardControl(c.Module, c.Command, &c.Parameters.Parameters, func(f *Face) bool { return f.cost != 0 })
		}
	case "lsa/flood":
		if *dummy || !this.isFreshLSA(params.LSA) {
			return
		}
		f.log("flood lsa", params.LSA.Id, "from", params.Uri)
		this.rib[params.LSA.Id] = params.LSA
		this.ribUpdated = true
		f.id = params.Uri
		this.flood(params.LSA, f)
	default:
		resp = RespNotSupported
	}
	return
}

func (this *Forwarder) forwardControl(module, command string, params *ndn.Parameters, validate func(*Face) bool) {
	control := new(ndn.ControlInterest)
	control.Name.Module = module
	control.Name.Command = command
	control.Name.Parameters.Parameters = *params
	i := new(ndn.Interest)
	ndn.Copy(control, i)
	for f := range this.face {
		if !validate(f) {
			continue
		}
		resp := make(chan (<-chan *ndn.Data))
		f.reqRecv <- &req{
			interest: i,
			resp:     resp,
		}
		<-resp
	}
}
