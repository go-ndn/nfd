package main

import (
	"encoding/json"
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/go-ndn/log"
	"github.com/go-ndn/ndn"
	"github.com/go-ndn/packet"
)

var (
	flagConfig = flag.String("config", "nfd.json", "config path")
	flagDebug  = flag.Bool("debug", false, "enable logging")
)

func main() {
	flag.Parse()

	// pprof
	go http.ListenAndServe(":6060", nil)

	// config
	configFile, err := os.Open(*flagConfig)
	if err != nil {
		log.Fatalln(err)
	}
	defer configFile.Close()
	err = json.NewDecoder(configFile).Decode(&config)
	if err != nil {
		log.Fatalln(err)
	}

	// key
	cert, err := os.Open(config.NDNCertPath)
	if err != nil {
		log.Fatalln(err)
	}
	defer cert.Close()
	key, err = ndn.DecodeCertificate(cert)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("key", key.Locator())

	// create faces
	for _, u := range config.Listen {
		ln, err := packet.Listen(u.Network, u.Address)
		if err != nil {
			log.Fatalln(err)
		}
		defer ln.Close()
		log.Println("listen", u.Network, u.Address)
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
