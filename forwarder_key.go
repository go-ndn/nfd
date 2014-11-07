package main

import (
	"github.com/taylorchu/ndn"
	"io/ioutil"
	"os"
)

func (this *Forwarder) decodePrivateKey(file string) (err error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	err = ndn.SignKey.DecodePrivateKey(b)
	return
}

func (this *Forwarder) decodeCertificate(file string) (err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	err = this.verifyKey.DecodeCertificate(f)
	return
}
