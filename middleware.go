package main

import (
	"sync"
	"time"

	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
)

const (
	loopIntv = time.Minute
)

var (
	// stop interest with same name and nonce from propagating
	loopChecker = func() mux.Middleware {
		var m loopMatcher
		var mu sync.Mutex
		return func(next mux.Handler) mux.Handler {
			return mux.HandlerFunc(func(w ndn.Sender, i *ndn.Interest) {
				var ok bool
				mu.Lock()
				m.Update(i.Name.Components, func(m map[uint64]struct{}) map[uint64]struct{} {
					if m == nil {
						m = make(map[uint64]struct{})
					} else {
						_, ok = m[i.Nonce]
					}
					if !ok {
						m[i.Nonce] = struct{}{}
					}
					return m
				}, false)
				mu.Unlock()
				if ok {
					return
				}

				time.AfterFunc(loopIntv, func() {
					mu.Lock()
					m.Update(i.Name.Components, func(m map[uint64]struct{}) map[uint64]struct{} {
						if m == nil {
							return nil
						}
						delete(m, i.Nonce)
						if len(m) == 0 {
							return nil
						}
						return m
					}, false)
					mu.Unlock()
				})
				next.ServeNDN(w, i)
			})
		}
	}()

	// create another namespace for local service
	cacher = mux.RawCacher(ndn.NewCache(65536), false)
)
