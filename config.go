package main

import (
	"encoding/json"
	"os"
)

type Url struct {
	Network, Address string
	Cost             uint64
}

type Config struct {
	Id              string
	Listen          []Url
	Remote          []Url
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
