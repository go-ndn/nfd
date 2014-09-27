package main

import (
	"flag"
	"github.com/taylorchu/exact"
	"github.com/taylorchu/lpm"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var (
	configPath = flag.String("c", "nfd.json", "nfd config file path")
)

func main() {
	flag.Parse()
	conf, err := NewConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	createFace := make(chan net.Conn)

	fw := &Forwarder{
		fib:        lpm.New(),
		fibNames:   make(map[*Face]map[string]bool),
		forwarded:  exact.New(),
		createFace: createFace,
	}
	err = fw.decodePrivateKey(conf.PrivateKeyPath)
	if err != nil {
		log.Fatal(err)
	}
	err = fw.decodeCertificate(conf.CertificatePath)
	if err != nil {
		log.Fatal(err)
	}
	go fw.Run()

	for _, u := range conf.LocalUrl {
		ln, err := net.Listen(u.Network, u.Address)
		if err != nil {
			log.Fatal(err)
		}
		defer ln.Close()
		log.Println("listening", u.Network, u.Address)
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					continue
				}
				createFace <- conn
			}
		}()
	}
	for _, u := range conf.RemoteUrl {
		for {
			// retry until connection established
			conn, err := net.Dial(u.Network, u.Address)
			if err != nil {
				continue
			}
			createFace <- conn
			break
		}
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("goodbye nfd")
}
