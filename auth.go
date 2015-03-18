package main

import (
	"io/ioutil"
	"os"

	"github.com/go-ndn/ndn"
)

var (
	verifyKey ndn.Key
	timestamp uint64
)

func decodePrivateKey(file string) (err error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	err = ndn.SignKey.DecodePrivateKey(b)
	return
}

func decodeCertificate(file string) (err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	err = verifyKey.DecodeCertificate(f)
	return
}
