package main

import "github.com/go-ndn/mux"

//go:generate gorewrite

func init() {
	fibNodeValEmpty = func(t map[uint64]mux.Handler) bool {
		return t == nil
	}

	loopNodeValEmpty = func(t map[uint64]struct{}) bool {
		return t == nil
	}
}
