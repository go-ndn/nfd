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
	f.Match(i.Name.String(), func(v interface{}) {
		for _, h := range v.(map[mux.Handler]mux.Handler) {
			h.ServeNDN(w, i)
			break
		}
	}, true)
}

func (f *fib) add(name string, h mux.Handler, mw ...mux.Middleware) {
	f.Println("add", name)
	h2 := h
	for _, m := range mw {
		h2 = m(h2)
	}
	f.Update(name, func(v interface{}) interface{} {
		var m map[mux.Handler]mux.Handler
		if v == nil {
			m = make(map[mux.Handler]mux.Handler)
		} else {
			m = v.(map[mux.Handler]mux.Handler)
		}
		m[h] = h2
		return m
	}, false)
}

func (f *fib) remove(name string, h mux.Handler) {
	f.Println("remove", name)
	f.Update(name, func(v interface{}) interface{} {
		if v == nil {
			return nil
		}
		m := v.(map[mux.Handler]mux.Handler)
		delete(m, h)
		if len(m) == 0 {
			return nil
		}
		return m
	}, false)
}
