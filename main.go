package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"github.com/taylorchu/exact"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var (
	configPath = flag.String("c", "nfd.json", "nfd config file path")
)

var (
	VerifyKey *ndn.Key
)

func decodePrivateKey(file string) (err error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	err = ndn.SignKey.DecodePrivateKey(b)
	return
}

func decodeCertificate(file string) (err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	var d ndn.Data
	err = d.ReadFrom(bufio.NewReader(base64.NewDecoder(base64.StdEncoding, f)))
	if err != nil {
		return
	}
	VerifyKey = new(ndn.Key)
	err = VerifyKey.DecodePublicKey(d.Content)
	return
}

func main() {
	flag.Parse()
	conf, err := NewConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	err = decodePrivateKey(conf.PrivateKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	err = decodeCertificate(conf.CertificatePath)
	if err != nil {
		log.Fatal(err)
	}

	createFace := make(chan net.Conn)
	go (&Forwarder{
		fib:        lpm.New(),
		fibNames:   make(map[*Face]map[string]bool),
		forwarded:  exact.New(),
		createFace: createFace,
	}).Run()

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
