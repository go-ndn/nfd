package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
)

func loopChecker(next mux.Handler) mux.Handler {
	m := newMatcher()
	return mux.HandlerFunc(func(w ndn.Sender, i *ndn.Interest) {
		interestID := fmt.Sprintf("%s/%x", i.Name, i.Nonce)
		if m.Match(interestID) {
			return
		}
		go func() {
			time.Sleep(time.Minute)
			m.Remove(interestID)
		}()
		next.ServeNDN(w, i)
	})
}

type matcher struct {
	m map[string]struct{}
	sync.Mutex
}

func newMatcher() *matcher {
	return &matcher{
		m: make(map[string]struct{}),
	}
}

func (m *matcher) Remove(s string) {
	m.Lock()
	delete(m.m, s)
	m.Unlock()
}

func (m *matcher) Match(s string) bool {
	m.Lock()
	defer m.Unlock()
	_, ok := m.m[s]
	if !ok {
		m.m[s] = struct{}{}
	}
	return ok
}
