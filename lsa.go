package main

import (
	"time"

	"github.com/go-ndn/lpm"
	"github.com/go-ndn/ndn"
)

func AddNextHop(name string, f *Face, local bool) {
	Fib.Update(lpm.Key(name), func(chs interface{}) interface{} {
		f.log("add nexthop", name)
		f.registered[name] = local
		if chs == nil {
			return map[chan<- *req]bool{f.reqRecv: true}
		}
		chs.(map[chan<- *req]bool)[f.reqRecv] = true
		return chs
	}, false)
}

func RemoveNextHop(name string, f *Face) {
	Fib.Update(lpm.Key(name), func(chs interface{}) interface{} {
		f.log("remove nexthop", name)
		delete(f.registered, name)
		m := chs.(map[chan<- *req]bool)
		delete(m, f.reqRecv)
		if len(m) == 0 {
			return nil
		}
		return chs
	}, false)
}

func UpdateFib() {
	// build face id map and remove indirectly registered name
	faceId := make(map[string]*Face)
	for f := range Faces {
		if f.id == "" {
			continue
		}
		for name, local := range f.registered {
			if !local {
				RemoveNextHop(name, f)
			}
		}
		faceId[f.id] = f
	}

	// copy rib
	state := []*ndn.LSA{CreateLSA()}
	for _, lsa := range Rib {
		state = append(state, lsa)
	}

	// remote prefix
	for name, n := range bestRouteByName(state, Id) {
		f, ok := faceId[n.Id]
		if !ok {
			continue
		}
		if _, ok := f.registered[name]; !ok {
			AddNextHop(name, f, false)
		}
	}
}

func CreateLSA() *ndn.LSA {
	lsa := &ndn.LSA{
		Id:      Id,
		Version: uint64(time.Now().UTC().UnixNano() / 1000000),
	}
	n := make(map[string]bool)
	for f := range Faces {
		if f.id != "" && f.cost != 0 {
			lsa.Neighbor = append(lsa.Neighbor, ndn.Neighbor{
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
		lsa.Name = append(lsa.Name, name)
	}
	return lsa
}

func IsLSANewer(lsa *ndn.LSA) bool {
	if lsa.Id == Id {
		return false
	}
	if old, ok := Rib[lsa.Id]; ok && old.Version >= lsa.Version {
		return false
	}
	return true
}

func RemoveExpiredLSA() {
	ver := uint64(time.Now().UTC().Add(-LSAExpireIntv).UnixNano() / 1000000)
	for id, lsa := range Rib {
		if lsa.Version < ver {
			RibUpdated = true
			delete(Rib, id)
		}
	}
}

func FloodLSA(lsa *ndn.LSA, sender *Face) {
	SendControl("lsa", "flood", &ndn.Parameters{
		Uri: Id,
		LSA: lsa,
	}, func(f *Face) bool { return f != sender })
}
