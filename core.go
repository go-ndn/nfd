package main

import (
	"net"
	"os"
	"time"

	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
	"github.com/sirupsen/logrus"
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

	trustedCert []ndn.Key
}

type request struct {
	ndn.Sender
	*ndn.Interest
}

func newCore(ctx *context) *core {
	nextHop := &fib{
		FieldLogger: logrus.WithField("module", "fib"),
	}
	// defaultCacher caches packets that are not generated locally.
	defaultCacher := mux.RawCacher(&mux.CacherOptions{
		Cache:       ndn.NewCache(65536),
		SkipPrivate: true,
	})
	// localCacher caches packets generated from local services.
	localCacher := mux.RawCacher(&mux.CacherOptions{
		Cache: ndn.NewCache(65536),
	})
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

	for _, path := range ctx.NDNCertPath {
		key, err := decodeCertificate(path)
		if err != nil {
			logrus.WithError(err).Error("decode certificate")
			continue
		}
		c.trustedCert = append(c.trustedCert, key)
	}

	// register local service
	c.registerService()

	c.HandleFunc("/", func(w ndn.Sender, i *ndn.Interest) error {
		go nextHop.ServeNDN(w, i)
		return nil
	}, loopChecker(time.Minute), defaultCacher)
	return c
}

func decodeCertificate(path string) (ndn.Key, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	key, err := ndn.DecodeCertificate(f)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (c *core) Accept(conn net.Conn) {
	c.faceCreate <- conn
}

func (c *core) newFaceID() uint64 {
	c.lastFaceID++
	return c.lastFaceID
}

func (c *core) Start() {
	for {
		select {
		case conn := <-c.faceCreate:
			c.addFace(conn)
		case faceID := <-c.faceClose:
			c.removeFace(faceID)
		case req := <-c.reqSend:
			c.ServeNDN(req.Sender, req.Interest)
		}
	}
}

func (c *core) addFace(conn net.Conn) {
	recv := make(chan *ndn.Interest, 1024)

	id := c.newFaceID()
	f := &face{
		Face:  ndn.NewFace(conn, recv),
		id:    id,
		route: make(map[string]ndn.Route),
		FieldLogger: logrus.WithFields(logrus.Fields{
			"face":   id,
			"remote": conn.RemoteAddr(),
		}),
	}

	c.face[f.id] = f

	go func() {
		serializer := mux.RawCacher(&mux.CacherOptions{
			Cache:       ndn.NewCache(1024),
			SkipPrivate: true,
		})(
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
		f.Info("face removed")
	}()
	f.Info("face created")
}

func (c *core) removeFace(faceID uint64) {
	f := c.face[faceID]
	delete(c.face, faceID)
	for name := range f.route {
		c.nextHop.remove(ndn.NewName(name), faceID)
	}
}
