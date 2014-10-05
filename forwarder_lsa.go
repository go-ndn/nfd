package main

import (
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
	"time"
)

func newLSA(id string) *ndn.LSA {
	return &ndn.LSA{
		Id:      id,
		Version: uint64(time.Now().UTC().UnixNano() / 1000000),
	}
}

func (this *Forwarder) updateFib(shortest map[string]ndn.Neighbor) *lpm.Matcher {
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
		f, ok := faceId[n.Id]
		if !ok {
			// neighbor face might be removed after calculation
			continue
		}
		update(name, f)
	}
	return fib
}

func (this *Forwarder) localLSA() *ndn.LSA {
	v := newLSA(this.id)
	n := make(map[string]bool)
	for f := range this.face {
		if f.id == "" || f.cost == 0 {
			continue
		}
		v.Neighbor = append(v.Neighbor, ndn.Neighbor{
			Id:   f.id,
			Cost: f.cost,
		})
		for name := range f.registered {
			n[name] = true
		}
	}
	for name := range n {
		v.Name = append(v.Name, name)
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

func (this *Forwarder) flood(id string, sender *Face) {
	v, ok := this.rib[id]
	if !ok {
		return
	}
	control := new(ndn.ControlInterest)
	control.Name.Module = "lsa"
	control.Name.Command = "flood"
	control.Name.Parameters.Parameters.Uri = this.id
	control.Name.Parameters.Parameters.LSA = *v
	i := new(ndn.Interest)
	ndn.Copy(control, i)
	for f := range this.face {
		if f == sender {
			continue
		}
		resp := make(chan (<-chan *ndn.Data))
		f.reqRecv <- &req{
			interest: i,
			sender:   f,
			resp:     resp,
		}
		<-resp
	}
}
