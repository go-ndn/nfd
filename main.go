package main

import (
	"flag"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var (
	ActiveFaces = make(map[*Face]bool)
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

	bcast := make(chan *InterestBcast)
	closed := make(chan *Face)
	create := make(chan net.Conn)

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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case conn := <-create:
			f := &Face{
				Face:      ndn.NewFace(conn),
				fib:       lpm.New(),
				closed:    closed,
				bcastSend: bcast,
				bcastRecv: make(chan *InterestBcast),
				dataOut:   make(chan *ndn.Data),
			}
			ActiveFaces[f] = true
			f.log("face created")
			go f.Listen()
		case b := <-bcast:
			// broadcast
			for f := range ActiveFaces {
				f.bcastRecv <- b
			}
		case f := <-closed:
			f.log("face removed")
			delete(ActiveFaces, f)
		case <-quit:
			log.Println("goodbye nfd")
			return
		}
	}
}
