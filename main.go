package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/taylorchu/exact"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
)

var (
	configPath = flag.String("config", "nfd.json", "config path")
	debug      = flag.Bool("debug", false, "enable logging")
	dummy      = flag.Bool("dummy", false, "disable routing and enable remote registration")
)

type connInfo struct {
	conn net.Conn
	cost uint64
}

func log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[core] %s", fmt.Sprintln(i...))
}

func main() {
	flag.Parse()
	// pprof
	log("http://localhost:6060/debug/pprof/")
	go http.ListenAndServe(":6060", nil)

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
	go fw.Run()

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
				fw.faceCreate <- &connInfo{conn: conn}
			}
		}()
	}
	for _, u := range conf.Remote {
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
