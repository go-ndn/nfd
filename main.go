package main

import (
	"bufio"
	"encoding/base64"
	"flag"
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

	activeFaces := make(map[*Face]bool)
	bcast := make(chan *interestBcast)
	bcastFib := make(chan *fibBcast)
	closed := make(chan *Face)
	create := make(chan net.Conn, len(conf.RemoteUrl))
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

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
					log.Println(err)
					continue
				}
				create <- conn
			}
		}()
	}

	for _, u := range conf.RemoteUrl {
		conn, err := net.Dial(u.Network, u.Address)
		if err != nil {
			log.Fatal(err)
		}
		create <- conn
	}

	for {
		select {
		case conn := <-create:
			f := &Face{
				Face:         ndn.NewFace(conn),
				fib:          lpm.New(),
				closed:       closed,
				bcastFibSend: bcastFib,
				bcastFibRecv: make(chan *fibBcast),
				bcastSend:    bcast,
				bcastRecv:    make(chan *interestBcast),
				dataOut:      make(chan *ndn.Data),
			}
			activeFaces[f] = true
			f.log("face created")
			go f.Listen()
		case b := <-bcast:
			// broadcast
			for f := range activeFaces {
				f.bcastRecv <- b
			}
		case b := <-bcastFib:
			for f := range activeFaces {
				f.bcastFibRecv <- b
			}
		case f := <-closed:
			f.log("face removed")
			delete(activeFaces, f)
		case <-quit:
			log.Println("goodbye nfd")
			return
		}
	}
}
