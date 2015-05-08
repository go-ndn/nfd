package main

import (
	"encoding/json"
	"flag"
	"net"
	"os"

	"github.com/go-ndn/packet"
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
	f, err = os.Open(config.NDNCertPath)
	if err != nil {
		log(err)
		return
	}
	defer f.Close()
	key.DecodeCertificate(f)
	log("key", key.Name)

	// create faces
	for _, u := range config.Listen {
		ln, err := listen(u.Network, u.Address)
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

func listen(network, address string) (net.Listener, error) {
	switch network {
	case "udp":
		fallthrough
	case "udp4":
		fallthrough
	case "udp6":
		fallthrough
	case "ip":
		fallthrough
	case "ip4":
		fallthrough
	case "ip6":
		fallthrough
	case "unixgram":
		return packet.Listen(network, address)
	default:
		return net.Listen(network, address)
	}
}
