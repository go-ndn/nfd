package main

import (
	"encoding/json"
	"os"
)

type URL struct {
	Network, Address string
}

type Config struct {
	Listen          []URL
	Remote          []URL
	CertificatePath string
	PrivateKeyPath  string
}

func NewConfig(file string) (c *Config, err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	c = new(Config)
	err = dec.Decode(c)
	return
}
