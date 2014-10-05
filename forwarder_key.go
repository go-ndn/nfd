package main

import (
	"bufio"
	"encoding/base64"
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
	var d ndn.Data
	err = d.ReadFrom(bufio.NewReader(base64.NewDecoder(base64.StdEncoding, f)))
	if err != nil {
		return
	}
	err = this.verifyKey.DecodePublicKey(d.Content)
	return
}
