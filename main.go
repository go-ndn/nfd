package main

import (
	"flag"
	"github.com/davecheney/profile"
	"github.com/taylorchu/exact"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var (
	configPath = flag.String("config", "nfd.json", "nfd config file path")
	debug      = flag.Bool("debug", false, "enable logging")
	dummy      = flag.Bool("dummy", false, "disable routing and enable remote registration")
)

func main() {
	flag.Parse()
	if *debug {
		defer profile.Start(profile.CPUProfile).Stop()
	}
	conf, err := NewConfig(*configPath)
	if err != nil {
		log(err)
		return
	}
	log("nfd id", conf.Id)
	if *dummy {
		log("routing disabled")
	}

	fw := &Forwarder{
		fib:        lpm.New(),
		forwarded:  exact.New(),
		faceCreate: make(chan *connInfo),
		face:       make(map[*Face]bool),
		id:         conf.Id,
		rib:        make(map[string]*ndn.LSA),
	}
	err = fw.decodePrivateKey(conf.PrivateKeyPath)
	if err != nil {
		log(err)
		return
	}
	err = fw.decodeCertificate(conf.CertificatePath)
	if err != nil {
		log(err)
		return
	}
	go fw.Run()

	for _, u := range conf.LocalUrl {
		ln, err := net.Listen(u.Network, u.Address)
		if err != nil {
			log(err)
			return
		}
		defer ln.Close()
		log("listening", u.Network, u.Address)
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					continue
				}
				fw.faceCreate <- &connInfo{conn: conn}
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
			fw.faceCreate <- &connInfo{
				conn: conn,
				cost: u.Cost,
			}
			break
		}
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log("goodbye nfd")
}
