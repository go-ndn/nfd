package main

import (
	"flag"
	"github.com/taylorchu/lpm"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Face struct {
	in     chan interface{}
	out    chan interface{}
	c      net.Conn
	closed bool
	pit    *lpm.Matcher
	fib    *lpm.Matcher
}

var (
	ActiveFaces  []*Face
	ContentStore = lpm.New()
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

	bcast := make(chan interface{})

	var m sync.RWMutex
	for _, u := range conf.LocalUrl {
		ln, err := net.Listen(u.Network, u.Address)
		if err != nil {
			log.Fatal(err)
		}
		defer ln.Close()
		log.Println("listening", u.Network, u.Address)
		go func(ln net.Listener) {
			for {
				c, err := ln.Accept()
				if err != nil {
					// handle error
					continue
				}
				f := &Face{
					in:  make(chan interface{}),
					out: bcast,
					c:   c,
					pit: lpm.New(),
					fib: lpm.New(),
				}
				m.Lock()
				ActiveFaces = append(ActiveFaces, f)
				m.Unlock()
				log.Println("face created", c.RemoteAddr())
				go f.Incoming()
				go f.Outgoing()
			}
		}(ln)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-quit:
			log.Println("goodbye, nfd")
			return
		case packet := <-bcast:
			// broadcast
			m.RLock()
			for _, f := range ActiveFaces {
				f.in <- packet
			}
			m.RUnlock()
		default:
			for i := len(ActiveFaces) - 1; i >= 0; i-- {
				if ActiveFaces[i].closed {
					log.Println("face removed", ActiveFaces[i].c.RemoteAddr())
					m.Lock()
					ActiveFaces = append(ActiveFaces[:i], ActiveFaces[i+1:]...)
					m.Unlock()
				}
			}
		}
	}
}
