package main

import (
	"flag"
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
				Face:   ndn.NewFace(conn),
				Closed: closed,
				Bcast:  bcast,
			}
			ActiveFaces[f] = true
			f.log("face created")
			go f.Listen()
		case b := <-bcast:
			// broadcast
			for f := range ActiveFaces {
				if f.Fib.Match(newLPMKey(b.Interest.Name)) == nil {
					continue
				}
				f.log("interest forwarded", b.Interest.Name)
				ch, err := f.SendInterest(b.Interest)
				if err != nil {
					log.Println(err)
					continue
				}
				go func() {
					d, ok := <-ch
					if !ok {
						return
					}
					b.Sender.log("data returned", d.Name)
					b.Sender.SendData(d)
				}()
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
