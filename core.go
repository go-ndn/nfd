package main

import (
	"fmt"
	"net"
	"time"

	"github.com/go-ndn/log"
	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
)

type core struct {
	face       map[uint64]*face
	lastFaceID uint64
	timestamp  uint64

	reqSend    chan request
	faceCreate chan net.Conn
	faceClose  chan uint64

	localCacher mux.Middleware
	verifier    mux.Handler
	nextHop     *fib
	*mux.Mux
}

type request struct {
	ndn.Sender
	*ndn.Interest
}

func newCore(ctx *context) *core {
	nextHop := newFIB(ctx)
	// defaultCacher caches packets that are not generated locally.
	defaultCacher := mux.RawCacher(ndn.NewCache(65536), false)
	// localCacher caches packets generated from local services.
	localCacher := mux.RawCacher(ndn.NewCache(65536), false)
	c := &core{
		face:        make(map[uint64]*face),
		reqSend:     make(chan request, 1024),
		faceCreate:  make(chan net.Conn),
		faceClose:   make(chan uint64),
		localCacher: localCacher,
		verifier:    mux.Verifier(ctx.Rule...)(defaultCacher(nextHop)),
		nextHop:     nextHop,
		Mux:         mux.New(),
	}
	// register local service
	c.registerService()

	// serve certificates
	for _, path := range ctx.NDNCertPath {
		name, h := mux.StaticFile(path)
		nextHop.add(ndn.NewName(name), 0, h)
	}

	c.HandleFunc("/", func(w ndn.Sender, i *ndn.Interest) error {
		go nextHop.ServeNDN(w, i)
		return nil
	}, loopChecker(time.Minute), defaultCacher)
	return c
}

func (c *core) Accept(conn net.Conn) {
	c.faceCreate <- conn
}

func (c *core) newFaceID() uint64 {
	c.lastFaceID++
	return c.lastFaceID
}

func (c *core) Start(ctx *context) {
	for {
		select {
		case conn := <-c.faceCreate:
			c.addFace(ctx, conn)
		case faceID := <-c.faceClose:
			c.removeFace(faceID)
		case req := <-c.reqSend:
			c.ServeNDN(req.Sender, req.Interest)
		}
	}
}

func (c *core) addFace(ctx *context, conn net.Conn) {
	recv := make(chan *ndn.Interest, 1024)

	f := &face{
		Face:  ndn.NewFace(conn, recv),
		id:    c.newFaceID(),
		route: make(map[string]ndn.Route),
	}

	if ctx.Debug {
		f.Logger = log.New(log.Stderr, fmt.Sprintf("[%s] ", conn.RemoteAddr()))
	} else {
		f.Logger = log.Discard
	}

	c.face[f.id] = f

	go func() {
		serializer := mux.RawCacher(ndn.NewCache(1024), false)(
			mux.HandlerFunc(func(w ndn.Sender, i *ndn.Interest) error {
				// serialize requests
				c.reqSend <- request{
					Sender:   w,
					Interest: i,
				}
				return nil
			}))
		for i := range recv {
			serializer.ServeNDN(f, i)
		}
		c.faceClose <- f.id
		f.Close()
		f.Println("face removed")
	}()
	f.Println("face created")
}

func (c *core) removeFace(faceID uint64) {
	f := c.face[faceID]
	delete(c.face, faceID)
	for name := range f.route {
		c.nextHop.remove(ndn.NewName(name), faceID)
	}
}
