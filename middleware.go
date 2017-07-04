package main

import (
	"sync"
	"time"

	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
)

// loopChecker stops interest with same name and nonce from propagating.
func loopChecker(d time.Duration) mux.Middleware {
	var m loopMatcher
	var mu sync.Mutex
	return func(next mux.Handler) mux.Handler {
		return mux.HandlerFunc(func(w ndn.Sender, i *ndn.Interest) error {
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
				return nil
			}

			time.AfterFunc(d, func() {
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
			return next.ServeNDN(w, i)
		})
	}
}
