package main

import (
	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
	"github.com/sirupsen/logrus"
)

type fib struct {
	fibMatcher
	logrus.FieldLogger
}

func (f *fib) ServeNDN(w ndn.Sender, i *ndn.Interest) error {
	f.Match(i.Name.Components, func(m map[uint64]mux.Handler) {
		for _, h := range m {
			h.ServeNDN(w, i)
			break
		}
	}, true)
	return nil
}

func (f *fib) add(name ndn.Name, id uint64, h mux.Handler) {
	f.WithFields(logrus.Fields{
		"name": name,
		"face": id,
	}).Info("add route")
	f.Update(name.Components, func(m map[uint64]mux.Handler) map[uint64]mux.Handler {
		if m == nil {
			m = make(map[uint64]mux.Handler)
		}
		m[id] = h
		return m
	}, false)
}

func (f *fib) remove(name ndn.Name, id uint64) {
	f.WithFields(logrus.Fields{
		"name": name,
		"face": id,
	}).Info("remove route")
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
