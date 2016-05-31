package main

import "github.com/go-ndn/mux"

//go:generate generic github.com/go-ndn/lpm/matcher .fib Type->github.com/go-ndn/mux:map[uint64]mux.Handler TypeMatcher->fibMatcher
//go:generate generic github.com/go-ndn/lpm/matcher .loop Type->map[uint64]struct{} TypeMatcher->loopMatcher

func init() {
	fibNodeValEmpty = func(t map[uint64]mux.Handler) bool {
		return t == nil
	}

	loopNodeValEmpty = func(t map[uint64]struct{}) bool {
		return t == nil
	}
}
