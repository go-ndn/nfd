package main

import (
	"fmt"
	"time"

	"github.com/go-ndn/lpm"
	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
)

const (
	loopIntv = time.Minute
)

var (
	// stop interest with same name and nonce from propagating
	loopChecker = func() mux.Middleware {
		m := lpm.NewThreadSafe()
		return func(next mux.Handler) mux.Handler {
			return mux.HandlerFunc(func(w ndn.Sender, i *ndn.Interest) {
				interestID := fmt.Sprintf("%s/%x", i.Name, i.Nonce)
				var ok bool
				m.Update(interestID, func(v interface{}) interface{} {
					ok = v == nil
					return struct{}{}
				}, false)
				if !ok {
					return
				}

				time.AfterFunc(loopIntv, func() {
					m.Update(interestID, func(interface{}) interface{} {
						return nil
					}, false)
				})
				next.ServeNDN(w, i)
			})
		}
	}()

	// create another namespace for local service
	cacher = mux.RawCacher(ndn.NewCache(65536), false)
)
