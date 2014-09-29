package main

import (
	"bufio"
	"encoding/base64"
	"github.com/taylorchu/exact"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
	"io/ioutil"
	"log"
	"net"
	"os"
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
			log.Println("recompute fib")
			this.fib = this.computeNextHop()
			this.ribUpdated = false
		case <-floodTimer:
			log.Println("flood lsa")
			resp := make(chan (<-chan *ndn.Data))
			for f := range this.face {
				f.reqRecv <- &req{
					interest: this.newFloodInterest(this.rib[this.id]),
					resp:     resp,
				}
				<-resp
			}
		case <-expireTimer:
			log.Println("remove expired lsa")
			this.removeExpiredLSA()
		case f := <-closed:
			delete(this.face, f)
			// prefix or neighbor id removed
			this.updateLSA(this.localLSA())
			f.log("face removed")
		}
	}
}

func newLSA(id string) *ndn.LSA {
	return &ndn.LSA{
		Id:      id,
		Version: uint64(time.Now().UTC().UnixNano() / 1000000),
	}
}

// TODO: multipath, currently only choose the best
func (this *Forwarder) computeNextHop() *lpm.Matcher {
	shortest := make(map[string]ndn.Neighbor)
	// create graph from lsa
	graph := make(map[string]distMap)
	for id, v := range this.rib {
		dist := make(distMap)
		for _, u := range v.Neighbor {
			dist[u.Id] = u.Cost
		}
		graph[id] = dist
	}
	// for each prefix, find a shortest neighbor to forward
	for n, dist := range computeMultiPath(this.id, graph) {
		// if neighbor face is chosen
		for u, cost := range dist {
			if u == this.id {
				continue
			}
			for _, name := range this.rib[u].Name {
				if s, ok := shortest[name]; ok && cost >= s.Cost {
					continue
				}
				shortest[name] = ndn.Neighbor{
					Id:   n,
					Cost: cost,
				}
			}
		}
	}
	// next fib
	fib := lpm.New()
	update := func(name string, f *Face) {
		fib.Update(lpm.Key(name), func(chs interface{}) interface{} {
			f.log("add-nexthop", name)
			if chs == nil {
				return map[chan<- *req]bool{f.reqRecv: true}
			}
			chs.(map[chan<- *req]bool)[f.reqRecv] = true
			return chs
		}, false)
	}
	// local prefix and build face id map
	faceId := make(map[string]*Face)
	for f := range this.face {
		for name := range f.registered {
			update(name, f)
		}
		if f.id == "" {
			continue
		}
		faceId[f.id] = f
	}
	// remote prefix
	for name, n := range shortest {
		update(name, faceId[n.Id])
	}
	return fib
}

func (this *Forwarder) localLSA() *ndn.LSA {
	v := newLSA(this.id)
	for f := range this.face {
		if f.id == "" {
			continue
		}
		v.Neighbor = append(v.Neighbor, ndn.Neighbor{
			Id:   f.id,
			Cost: f.cost,
		})
		for name := range f.registered {
			v.Name = append(v.Name, name)
		}
	}
	return v
}

func (this *Forwarder) canFlood(v *ndn.LSA) bool {
	if v.Id == this.id {
		return false
	}
	if old, ok := this.rib[v.Id]; ok && old.Version >= v.Version {
		return false
	}
	return true
}

func (this *Forwarder) updateLSA(v *ndn.LSA) {
	this.rib[v.Id] = v
	this.ribUpdated = true
}

func (this *Forwarder) removeExpiredLSA() {
	ver := uint64(time.Now().UTC().Add(-ExpireTimer).UnixNano() / 1000000)
	for id, v := range this.rib {
		if v.Version < ver && v.Id != this.id {
			this.ribUpdated = true
			delete(this.rib, id)
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
	this.forwarded.Update(exact.Key(b.interest.Name.String()+string(b.interest.Nonce)), func(v interface{}) interface{} {
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
		return true
	})
}

func (this *Forwarder) decodePrivateKey(file string) (err error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	err = ndn.SignKey.DecodePrivateKey(b)
	return
}

func (this *Forwarder) decodeCertificate(file string) (err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	var d ndn.Data
	err = d.ReadFrom(bufio.NewReader(base64.NewDecoder(base64.StdEncoding, f)))
	if err != nil {
		return
	}
	err = this.verifyKey.DecodePublicKey(d.Content)
	return
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
			// neighbor id learned
			f.id = params.Uri
			this.updateLSA(this.localLSA())
		}
		for other := range this.face {
			if f == other {
				continue
			}
			resp := make(chan (<-chan *ndn.Data))
			other.reqRecv <- &req{
				interest: this.newFloodInterest(&params.LSA),
				sender:   f,
				resp:     resp,
			}
			<-resp
		}
	default:
		resp = RespNotSupported
	}
	return
}

func (this *Forwarder) newFloodInterest(v *ndn.LSA) *ndn.Interest {
	control := new(ndn.ControlInterest)
	control.Name.Module = "lsa"
	control.Name.Command = "flood"
	control.Name.Parameters.Parameters.Uri = this.id
	control.Name.Parameters.Parameters.LSA = *v
	i := new(ndn.Interest)
	ndn.Copy(control, i)
	return i
}
