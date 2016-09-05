package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/go-ndn/log"
	"github.com/go-ndn/packet"
)

func main() {
	ctx, err := background()
	if err != nil {
		log.Println(err)
		return
	}

	// pprof
	go http.ListenAndServe(":6060", nil)

	c := newCore(ctx)
	// create faces
	for _, u := range ctx.Listen {
		ln, err := packet.Listen(u.Network, u.Address)
		if err != nil {
			log.Println(err)
			return
		}
		defer ln.Close()
		log.Println("listen", u.Network, u.Address)
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					continue
				}
				c.Accept(conn)
			}
		}()
	}
	c.Start(ctx)
}
