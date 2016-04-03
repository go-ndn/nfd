package main

import (
	"github.com/go-ndn/log"
	"github.com/go-ndn/lpm"
	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
)

type fib struct {
	lpm.Matcher
	log.Logger
}

func newFIB() *fib {
	f := &fib{Matcher: lpm.New()}
	if *flagDebug {
		f.Logger = log.New(log.Stderr, "[fib] ")
	} else {
		f.Logger = log.Discard
	}
	return f
}

func (f *fib) ServeNDN(w ndn.Sender, i *ndn.Interest) {
	f.Match(i.Name.Components, func(v interface{}) {
		for _, h := range v.(map[uint64]mux.Handler) {
			h.ServeNDN(w, i)
			break
		}
	}, true)
}

func (f *fib) add(name ndn.Name, id uint64, h mux.Handler, mw ...mux.Middleware) {
	f.Println("add", name)
	for _, m := range mw {
		h = m(h)
	}
	f.Update(name.Components, func(v interface{}) interface{} {
		var m map[uint64]mux.Handler
		if v == nil {
			m = make(map[uint64]mux.Handler)
		} else {
			m = v.(map[uint64]mux.Handler)
		}
		m[id] = h
		return m
	}, false)
}

func (f *fib) remove(name ndn.Name, id uint64) {
	f.Println("remove", name)
	f.Update(name.Components, func(v interface{}) interface{} {
		if v == nil {
			return nil
		}
		m := v.(map[uint64]mux.Handler)
		delete(m, id)
		if len(m) == 0 {
			return nil
		}
		return m
	}, false)
}
