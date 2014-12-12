package main

import (
	"io/ioutil"
	"os"

	"github.com/taylorchu/ndn"
)

var (
	VerifyKey ndn.Key
	Timestamp uint64
)

func DecodePrivateKey(file string) (err error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	err = ndn.SignKey.DecodePrivateKey(b)
	return
}

func DecodeCertificate(file string) (err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	err = VerifyKey.DecodeCertificate(f)
	return
}
