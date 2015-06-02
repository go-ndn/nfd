package main

import (
	"encoding/json"
	"flag"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/go-ndn/ndn"
	"github.com/go-ndn/packet"
)

var (
	configPath = flag.String("config", "nfd.json", "config path")
	debug      = flag.Bool("debug", false, "enable logging")
)

func main() {
	// pprof
	go http.ListenAndServe(":6060", nil)

	flag.Parse()

	// config
	configFile, err := os.Open(*configPath)
	if err != nil {
		log(err)
		return
	}
	defer configFile.Close()
	err = json.NewDecoder(configFile).Decode(&config)
	if err != nil {
		log(err)
		return
	}

	// key
	cert, err := os.Open(config.NDNCertPath)
	if err != nil {
		log(err)
		return
	}
	defer cert.Close()
	key, err = ndn.DecodeCertificate(cert)
	if err != nil {
		log(err)
		return
	}
	log("key", key.Locator())

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
	case "udp", "udp4", "udp6", "ip", "ip4", "ip6", "unixgram":
		return packet.Listen(network, address)
	default:
		return net.Listen(network, address)
	}
}
