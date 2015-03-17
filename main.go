package main

import (
	"flag"
	"net"

	"github.com/go-ndn/ndn"
)

var (
	configPath = flag.String("config", "nfd.json", "config path")
	debug      = flag.Bool("debug", false, "enable logging")
)

func main() {
	flag.Parse()

	// config
	conf, err := NewConfig(*configPath)
	if err != nil {
		log(err)
		return
	}

	// key
	err = DecodePrivateKey(conf.PrivateKeyPath)
	if err != nil {
		log(err)
		return
	}
	log("signKey", ndn.SignKey.Name)
	err = DecodeCertificate(conf.CertificatePath)
	if err != nil {
		log(err)
		return
	}
	log("verifyKey", VerifyKey.Name)

	// create faces
	for _, url := range conf.Listen {
		ln, err := net.Listen(url.Network, url.Address)
		if err != nil {
			log(err)
			return
		}
		defer ln.Close()
		log("listen", url.Network, url.Address)
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					continue
				}
				FaceCreate <- conn
			}
		}()
	}
	for _, url := range conf.Remote {
		go func(url URL) {
			for {
				// retry until connection established
				conn, err := net.Dial(url.Network, url.Address)
				if err != nil {
					continue
				}
				FaceCreate <- conn
				break
			}
		}(url)
	}

	handleLocal()
	Run()
}
