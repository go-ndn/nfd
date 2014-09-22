package main

import (
	"encoding/json"
	"os"
)

type Url struct {
	Network, Address string
}

type Config struct {
	LocalUrl        []Url
	RemoteUrl       []Url
	CertificatePath string
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
