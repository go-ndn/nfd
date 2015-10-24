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
		for h := range v.(map[mux.Handler]struct{}) {
			h.ServeNDN(w, i)
			break
		}
	}, true)
}

func (f *fib) add(name string, h mux.Handler) {
	f.Println("add", name)
	f.Update(name, func(v interface{}) interface{} {
		var m map[mux.Handler]struct{}
		if v == nil {
			m = make(map[mux.Handler]struct{})
		} else {
			m = v.(map[mux.Handler]struct{})
		}
		m[h] = struct{}{}
		return m
	}, false)
}

func (f *fib) remove(name string, h mux.Handler) {
	f.Println("remove", name)
	f.Update(name, func(v interface{}) interface{} {
		if v == nil {
			return nil
		}
		m := v.(map[mux.Handler]struct{})
		delete(m, h)
		if len(m) == 0 {
			return nil
		}
		return m
	}, false)
}
