package main

import (
	"github.com/go-ndn/log"
	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
)

type fib struct {
	fibMatcher
	log.Logger
}

func newFIB() *fib {
	f := new(fib)
	if *flagDebug {
		f.Logger = log.New(log.Stderr, "[fib] ")
	} else {
		f.Logger = log.Discard
	}
	return f
}

func (f *fib) ServeNDN(w ndn.Sender, i *ndn.Interest) {
	f.Match(i.Name.Components, func(m map[uint64]mux.Handler) {
		for _, h := range m {
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
	f.Update(name.Components, func(m map[uint64]mux.Handler) map[uint64]mux.Handler {
		if m == nil {
			m = make(map[uint64]mux.Handler)
		}
		m[id] = h
		return m
	}, false)
}

func (f *fib) remove(name ndn.Name, id uint64) {
	f.Println("remove", name)
	f.Update(name.Components, func(m map[uint64]mux.Handler) map[uint64]mux.Handler {
		if m == nil {
			return nil
		}
		delete(m, id)
		if len(m) == 0 {
			return nil
		}
		return m
	}, false)
}
