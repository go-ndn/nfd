package main

import (
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
	"time"
)

func (this *Forwarder) addNextHop(name string, f *Face, local bool) {
	f.registered[name] = local
	this.fib.Update(lpm.Key(name), func(chs interface{}) interface{} {
		f.log("add-nexthop", name)
		if chs == nil {
			return map[chan<- *req]bool{f.reqRecv: true}
		}
		chs.(map[chan<- *req]bool)[f.reqRecv] = true
		return chs
	}, false)
}

func (this *Forwarder) removeNextHop(name string, f *Face) {
	delete(f.registered, name)
	this.fib.Update(lpm.Key(name), func(chs interface{}) interface{} {
		f.log("remove-nexthop", name)
		m := chs.(map[chan<- *req]bool)
		delete(m, f.reqRecv)
		if len(m) == 0 {
			return nil
		}
		return chs
	}, false)
}

func (this *Forwarder) updateFib(shortest map[string]ndn.Neighbor) {
	// build face id map
	faceId := make(map[string]*Face)
	for f := range this.face {
		if f.id == "" {
			continue
		}
		for name, local := range f.registered {
			if !local {
				this.removeNextHop(name, f)
			}
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
		if _, ok := f.registered[name]; !ok {
			this.addNextHop(name, f, false)
		}
	}
}

func (this *Forwarder) localLSA() *ndn.LSA {
	v := &ndn.LSA{
		Id:      this.id,
		Version: uint64(time.Now().UTC().UnixNano() / 1000000),
	}
	n := make(map[string]bool)
	for f := range this.face {
		if f.id == "" || f.cost == 0 {
			continue
		}
		v.Neighbor = append(v.Neighbor, ndn.Neighbor{
			Id:   f.id,
			Cost: f.cost,
		})
		for name, local := range f.registered {
			if local {
				n[name] = true
			}
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

func (this *Forwarder) removeExpiredLSA() {
	ver := uint64(time.Now().UTC().Add(-ExpireTimer).UnixNano() / 1000000)
	for id, v := range this.rib {
		if v.Version < ver && v.Id != this.id {
			this.ribUpdated = true
			delete(this.rib, id)
		}
	}
}

func (this *Forwarder) transferCommand(c *ndn.Command) {
	control := new(ndn.ControlInterest)
	control.Name.Module = c.Module
	control.Name.Command = c.Command
	control.Name.Parameters = c.Parameters
	i := new(ndn.Interest)
	ndn.Copy(control, i)
	for f := range this.face {
		if f.cost == 0 {
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

func (this *Forwarder) flood(v *ndn.LSA, sender *Face) {
	control := new(ndn.ControlInterest)
	control.Name.Module = "lsa"
	control.Name.Command = "flood"
	control.Name.Parameters.Parameters.Uri = this.id
	control.Name.Parameters.Parameters.LSA = v
	i := new(ndn.Interest)
	ndn.Copy(control, i)
	for f := range this.face {
		if f == sender {
			continue
		}
		resp := make(chan (<-chan *ndn.Data))
		f.reqRecv <- &req{
			interest: i,
			sender:   sender,
			resp:     resp,
		}
		<-resp
	}
}
