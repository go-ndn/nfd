package main

import (
	"flag"
	"net"
)

var (
	configPath = flag.String("config", "nfd.json", "config path")
	debug      = flag.Bool("debug", false, "enable logging")
)

func main() {
	flag.Parse()

	// config
	conf, err := newConfig(*configPath)
	if err != nil {
		log(err)
		return
	}

	// key
	err = decodePrivateKey(conf.PrivateKeyPath)
	if err != nil {
		log(err)
		return
	}
	log("key", key.Name)

	// create faces
	for _, u := range conf.Listen {
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
	for _, u := range conf.Remote {
		go func(network, address string) {
			for {
				// retry until connection established
				conn, err := net.Dial(network, address)
				if err != nil {
					continue
				}
				faceCreate <- conn
				break
			}
		}(u.Network, u.Address)
	}

	run()
}
