package main

import (
	"encoding/json"
	"os"
)

type url struct {
	Network, Address string
}

type config struct {
	Listen          []url
	Remote          []url
	CertificatePath string
	PrivateKeyPath  string
}

func newConfig(file string) (conf *config, err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	conf = new(config)
	err = dec.Decode(conf)
	return
}
