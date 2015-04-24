package main

import (
	"encoding/json"
	"os"
)

type url struct {
	Network, Address string
}

type config struct {
	Listen         []url
	Remote         []url
	PrivateKeyPath string
}

func newConfig(file string) (conf *config, err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	conf = new(config)
	err = json.NewDecoder(f).Decode(conf)
	return
}
