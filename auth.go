package main

import (
	"io/ioutil"

	"github.com/go-ndn/ndn"
)

var (
	key       ndn.Key
	timestamp uint64
)

func decodePrivateKey(file string) (err error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	err = key.DecodePrivateKey(b)
	return
}
