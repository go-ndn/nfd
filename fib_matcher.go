package main

import (
	"github.com/go-ndn/lpm"
	"github.com/go-ndn/mux"
)

type fibMatcher struct{ fibNode }

var fibNodeValEmpty func(map[uint64]mux.Handler) bool

type fibNode struct {
	val   map[uint64]mux.Handler
	table map[string]fibNode
}

func (n *fibNode) empty() bool {
	return fibNodeValEmpty(n.val) && len(n.table) == 0
}
func (n *fibNode) update(key []lpm.Component, depth int, f func([]lpm.Component, map[uint64]mux.Handler) map[uint64]mux.Handler, exist, all bool) {
	try := func() {
		if !exist || !fibNodeValEmpty(n.val) {
			n.val = f(key[:depth], n.val)
		}
	}
	if len(key) == depth {
		try()
		return
	}
	if n.table == nil {
		if exist {
			try()
			return
		}
		n.table = make(map[string]fibNode)
	}
	v, ok := n.table[string(key[depth])]
	if !ok {
		if exist {
			try()
			return
		}
	}
	if all {
		try()
	}
	v.update(key, depth+1, f, exist, all)
	if v.empty() {
		delete(n.table, string(key[depth]))
	} else {
		n.table[string(key[depth])] = v
	}
}
func (n *fibNode) match(key []lpm.Component, depth int, f func(map[uint64]mux.Handler), exist bool) {
	try := func() {
		if !exist || !fibNodeValEmpty(n.val) {
			f(n.val)
		}
	}
	if len(key) == depth {
		try()
		return
	}
	if n.table == nil {
		if exist {
			try()
		}
		return
	}
	v, ok := n.table[string(key[depth])]
	if !ok {
		if exist {
			try()
		}
		return
	}
	v.match(key, depth+1, f, exist)
}
func (n *fibNode) visit(key []lpm.Component, f func([]lpm.Component, map[uint64]mux.Handler) map[uint64]mux.Handler) {
	if !fibNodeValEmpty(n.val) {
		n.val = f(key, n.val)
	}
	for k, v := range n.table {
		v.visit(append(key, lpm.Component(k)), f)
		if v.empty() {
			delete(n.table, k)
		} else {
			n.table[k] = v
		}
	}
}
func (n *fibNode) Update(key []lpm.Component, f func(map[uint64]mux.Handler) map[uint64]mux.Handler, exist bool) {
	n.update(key, 0, func(_ []lpm.Component, v map[uint64]mux.Handler) map[uint64]mux.Handler {
		return f(v)
	}, exist, false)
}
func (n *fibNode) UpdateAll(key []lpm.Component, f func([]lpm.Component, map[uint64]mux.Handler) map[uint64]mux.Handler, exist bool) {
	n.update(key, 0, f, exist, true)
}
func (n *fibNode) Match(key []lpm.Component, f func(map[uint64]mux.Handler), exist bool) {
	n.match(key, 0, f, exist)
}
func (n *fibNode) Visit(f func([]lpm.Component, map[uint64]mux.Handler) map[uint64]mux.Handler) {
	key := make([]lpm.Component, 0, 16)
	n.visit(key, f)
}
