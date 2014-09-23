package main

import (
	"flag"
	"github.com/taylorchu/ndn"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
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
	var m sync.RWMutex

	createFace := func(conn net.Conn) {
		f := &Face{
			Face:   ndn.NewFace(conn),
			Closed: closed,
			Bcast:  bcast,
		}
		m.Lock()
		ActiveFaces[f] = true
		m.Unlock()
		f.log("face created")
		f.Listen()
	}

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
				go createFace(conn)
			}
		}()
	}

	for _, u := range conf.RemoteUrl {
		conn, err := net.Dial(u.Network, u.Address)
		if err != nil {
			log.Fatal(err)
		}
		go createFace(conn)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-quit:
			log.Println("goodbye nfd")
			return
		case b := <-bcast:
			// broadcast
			m.RLock()
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
			m.RUnlock()
		case f := <-closed:
			f.log("face removed")
			m.Lock()
			delete(ActiveFaces, f)
			m.Unlock()
		}
	}
}
