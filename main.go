package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net"
	"os"
)

var (
	configPath = flag.String("config", "nfd.json", "config path")
	debug      = flag.Bool("debug", false, "enable logging")
)

func main() {
	flag.Parse()

	// config
	f, err := os.Open(*configPath)
	if err != nil {
		log(err)
		return
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		log(err)
		return
	}

	// key
	pem, err := ioutil.ReadFile(config.PrivateKeyPath)
	if err != nil {
		log(err)
		return
	}
	key.DecodePrivateKey(pem)
	log("key", key.Name)

	// create faces
	for _, u := range config.Listen {
		ln, err := net.Listen(u.Network, u.Address)
		if err != nil {
			log(err)
			return
		}
		defer ln.Close()
		log("listen", u.Network, u.Address)
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					continue
				}
				faceCreate <- conn
			}
		}()
	}

	run()
}
