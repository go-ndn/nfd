package main

import (
	"github.com/go-ndn/lpm"
	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
)

type fib struct {
	m lpm.Matcher
}

func newFIB() *fib {
	return &fib{m: lpm.New()}
}

func (f *fib) ServeNDN(w mux.Sender, i *ndn.Interest) {
	f.m.Match(i.Name.String(), func(v interface{}) {
		for h := range v.(map[mux.Handler]struct{}) {
			log("forward", i.Name)
			h.ServeNDN(w, i)
			break
		}
	}, true)
}

func (f *fib) add(name string, h mux.Handler, childInherit bool) {
	updater := func(v interface{}) interface{} {
		var m map[mux.Handler]struct{}
		if v == nil {
			m = make(map[mux.Handler]struct{})
		} else {
			m = v.(map[mux.Handler]struct{})
		}
		m[h] = struct{}{}
		return m
	}
	if childInherit {
		f.m.UpdateAll(name, func(_ string, v interface{}) interface{} {
			return updater(v)
		}, false)
	} else {
		f.m.Update(name, updater, false)
	}
}

func (f *fib) remove(name string, h mux.Handler, childInherit bool) {
	updater := func(v interface{}) interface{} {
		if v == nil {
			return nil
		}
		m := v.(map[mux.Handler]struct{})
		delete(m, h)
		if len(m) == 0 {
			return nil
		}
		return m
	}
	if childInherit {
		f.m.UpdateAll(name, func(_ string, v interface{}) interface{} {
			return updater(v)
		}, false)
	} else {
		f.m.Update(name, updater, false)
	}
}
