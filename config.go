package main

import (
	"code.google.com/p/go-uuid/uuid"
	"encoding/json"
	"os"
)

type LocalUrl struct {
	Network, Address string
}

type RemoteUrl struct {
	Network, Address string
	Cost             uint64
}

type Config struct {
	Id              string
	LocalUrl        []LocalUrl
	RemoteUrl       []RemoteUrl
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
	if err != nil {
		return
	}
	if c.Id == "" {
		c.Id = uuid.New()
	}
	return
}
