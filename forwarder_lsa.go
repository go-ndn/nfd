package main

import (
	"time"

	"github.com/go-ndn/lpm"
	"github.com/go-ndn/ndn"
)

func (this *Forwarder) addNextHop(name string, f *Face, local bool) {
	f.registered[name] = local
	this.fib.Update(lpm.Key(name), func(chs interface{}) interface{} {
		f.log("add nexthop", name)
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
		f.log("remove nexthop", name)
		m := chs.(map[chan<- *req]bool)
		delete(m, f.reqRecv)
		if len(m) == 0 {
			return nil
		}
		return chs
	}, false)
}

func (this *Forwarder) bestRoute() <-chan map[string]ndn.Neighbor {
	// copy rib
	state := []*ndn.LSA{this.createLSA()}
	for _, v := range this.rib {
		state = append(state, v)
	}
	ch := make(chan map[string]ndn.Neighbor, 1)
	go func() {
		ch <- bestRouteByName(state, this.id)
		close(ch)
	}()
	return ch
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

func (this *Forwarder) createLSA() *ndn.LSA {
	v := &ndn.LSA{
		Id:      this.id,
		Version: uint64(time.Now().UTC().UnixNano() / 1000000),
	}
	n := make(map[string]bool)
	for f := range this.face {
		if f.id != "" && f.cost != 0 {
			v.Neighbor = append(v.Neighbor, ndn.Neighbor{
				Id:   f.id,
				Cost: f.cost,
			})
		}
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

func (this *Forwarder) isFreshLSA(v *ndn.LSA) bool {
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
		if v.Version < ver {
			this.ribUpdated = true
			delete(this.rib, id)
		}
	}
}

func (this *Forwarder) flood(v *ndn.LSA, sender *Face) {
	this.forwardControl("lsa", "flood", &ndn.Parameters{
		Uri: this.id,
		LSA: v,
	}, func(f *Face) bool { return f != sender })
}
