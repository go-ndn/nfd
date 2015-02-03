package main

import (
	"flag"
	"net"

	"github.com/go-ndn/ndn"
)

var (
	configPath = flag.String("config", "nfd.json", "config path")
	debug      = flag.Bool("debug", false, "enable logging")
	dummy      = flag.Bool("dummy", false, "disable routing and enable remote registration")
)

func main() {
	flag.Parse()

	// config
	conf, err := NewConfig(*configPath)
	if err != nil {
		log(err)
		return
	}
	if conf.Id != "" {
		Id = conf.Id
	}
	log("nfd id", Id)
	if *dummy {
		log("routing disabled")
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
				FaceCreate <- &connReq{conn: conn}
			}
		}()
	}
	for _, u := range conf.Remote {
		go func(u Url) {
			for {
				// retry until connection established
				conn, err := net.Dial(u.Network, u.Address)
				if err != nil {
					continue
				}
				FaceCreate <- &connReq{conn: conn, cost: u.Cost}
				break
			}
		}(u)
	}

	// main loop
	Run()
}
