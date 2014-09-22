package main

import (
	"github.com/taylorchu/lpm"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Listener struct {
	Network, Address string
}

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

func main() {
	bcast := make(chan interface{})

	var m sync.Mutex
	for _, l := range []Listener{
		//{"tcp", ":6363"},
		//{"udp", ":6363"},
		{"unix", "/var/run/nfd.sock"},
	} {
		ln, err := net.Listen(l.Network, l.Address)
		if err != nil {
			log.Println(err)
			continue
		}
		defer ln.Close()
		log.Println("listening", l.Network, l.Address)
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
			for _, f := range ActiveFaces {
				f.in <- packet
			}
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
