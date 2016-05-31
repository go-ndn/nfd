package main

import "github.com/go-ndn/lpm"

type loopMatcher struct {
	loopNode
}

var loopNodeValEmpty func(map[uint64]struct{}) bool

type loopNode struct {
	val   map[uint64]struct{}
	table map[string]loopNode
}

func (n *loopNode) empty() bool {
	return loopNodeValEmpty(n.val) && len(n.table) == 0
}

func (n *loopNode) update(key []lpm.Component, depth int, f func([]lpm.Component, map[uint64]struct{}) map[uint64]struct{}, exist, all bool) {
	try := func() {
		if depth == 0 {
			return
		}
		if !exist || !loopNodeValEmpty(n.val) {
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
		n.table = make(map[string]loopNode)
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

func (n *loopNode) match(key []lpm.Component, depth int, f func(map[uint64]struct{}), exist bool) {
	try := func() {
		if depth == 0 {
			return
		}
		if !exist || !loopNodeValEmpty(n.val) {
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

func (n *loopNode) visit(key []lpm.Component, f func([]lpm.Component, map[uint64]struct{}) map[uint64]struct{}) {
	if !loopNodeValEmpty(n.val) {
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

func (n *loopNode) Update(key []lpm.Component, f func(map[uint64]struct{}) map[uint64]struct{}, exist bool) {
	n.update(key, 0, func(_ []lpm.Component, v map[uint64]struct{}) map[uint64]struct{} {
		return f(v)
	}, exist, false)
}

func (n *loopNode) UpdateAll(key []lpm.Component, f func([]lpm.Component, map[uint64]struct{}) map[uint64]struct{}, exist bool) {
	n.update(key, 0, f, exist, true)
}

func (n *loopNode) Match(key []lpm.Component, f func(map[uint64]struct{}), exist bool) {
	n.match(key, 0, f, exist)
}

func (n *loopNode) Visit(f func([]lpm.Component, map[uint64]struct{}) map[uint64]struct{}) {
	key := make([]lpm.Component, 0, 16)
	n.visit(key, f)
}
