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
	Forwarded = make(map[string]bool) // check for interest loop
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

	create := make(chan net.Conn)

	go func() {
		activeFaces := make(map[*Face]bool)
		bcastSend := make(chan *bcast)
		closed := make(chan *Face)
		for {
			select {
			case conn := <-create:
				ch := make(chan *ndn.Interest)
				f := &Face{
					Face:       ndn.NewFace(conn, ch),
					fib:        lpm.New(),
					closed:     closed,
					bcastSend:  bcastSend,
					bcastRecv:  make(chan *bcast),
					interestIn: ch,
					dataOut:    make(chan *ndn.Data),
				}
				activeFaces[f] = true
				f.log("face created")
				go f.Listen()
			case b := <-bcastSend:
				for f := range activeFaces {
					f.bcastRecv <- b
				}
			case f := <-closed:
				delete(activeFaces, f)
				f.log("face removed")
			}
		}
	}()

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
		for {
			// retry until connection established
			conn, err := net.Dial(u.Network, u.Address)
			if err != nil {
				log.Println(err)
				continue
			}
			create <- conn
			break
		}
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("goodbye nfd")
}
