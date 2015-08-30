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

func loopChecker(next mux.Handler) mux.Handler {
	m := lpm.NewThreadSafe()
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
		go func() {
			time.Sleep(loopIntv)
			m.Update(interestID, func(interface{}) interface{} {
				return nil
			}, false)
		}()
		next.ServeNDN(w, i)
	})
}
