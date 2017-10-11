package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/go-ndn/packet"
	"github.com/sirupsen/logrus"
)

func main() {
	ctx, err := background()
	if err != nil {
		logrus.Error(err)
		return
	}
	if ctx.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// pprof
	go http.ListenAndServe(":6060", nil)

	c := newCore(ctx)
	// create faces
	for _, u := range ctx.Listen {
		log := logrus.WithFields(logrus.Fields{
			"net":  u.Network,
			"addr": u.Address,
		})
		ln, err := packet.Listen(u.Network, u.Address)
		if err != nil {
			log.Error(err)
			return
		}
		defer ln.Close()
		log.Info("listen")
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
	c.Start()
}
