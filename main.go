package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"github.com/taylorchu/exact"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

const (
	bufSize = 32
)

var (
	configPath = flag.String("c", "nfd.json", "nfd config file path")
)

var (
	VerifyKey *ndn.Key
	Forwarded = exact.New() // check for interest loop
	Fib       = lpm.New()
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

	create := make(chan net.Conn, bufSize)

	go func() {
		bcastSend := make(chan *bcast, bufSize)
		closed := make(chan *Face, bufSize)
		for {
			select {
			case conn := <-create:
				ch := make(chan *ndn.Interest, bufSize)
				f := &Face{
					Face:       ndn.NewFace(conn, ch),
					fibNames:   make(map[string]bool),
					bcastSend:  bcastSend,
					bcastRecv:  make(chan *bcast, bufSize),
					interestIn: ch,
					dataOut:    make(chan *ndn.Data, bufSize),
					closed:     closed,
				}
				f.log("face created")
				go f.Listen()
			case b := <-bcastSend:
				c := new(ndn.ControlInterest)
				err := ndn.Copy(&b.interest, c)
				if err == nil {
					// do not forward command to other faces
					d := &ndn.Data{
						Name: b.interest.Name,
					}
					d.Content, err = ndn.Marshal(handleCommand(&c.Name, b.sender), 101)
					if err != nil {
						continue
					}
					b.sender.dataOut <- d
					continue
				}
				chs := Fib.Match(b.interest.Name)
				if chs == nil {
					continue
				}
				for ch := range chs.(map[chan<- *bcast]bool) {
					// every face gets a fresh copy of bcast job
					// each bcast contains its own fetching pipeline, so it should not be shared
					bcastCopy := *b
					ch <- &bcastCopy
				}
			case f := <-closed:
				for nextHop := range f.fibNames {
					removeNextHop(ndn.NewName(nextHop), f)
				}
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

func handleCommand(c *ndn.Command, f *Face) (resp *ndn.ControlResponse) {
	if VerifyKey.Verify(c, c.SignatureValue.SignatureValue) != nil {
		resp = RespNotAuthorized
		return
	}
	resp = RespOK
	params := c.Parameters.Parameters
	switch c.Module + "." + c.Command {
	case "fib.add-nexthop":
		addNextHop(params.Name, f)
	case "fib.remove-nexthop":
		removeNextHop(params.Name, f)
	default:
		resp = RespNotSupported
	}
	return
}

func addNextHop(name ndn.Name, f *Face) {
	Fib.Update(name, func(chs interface{}) interface{} {
		f.log("add-nexthop", name)
		f.fibNames[name.String()] = true
		if chs == nil {
			return map[chan<- *bcast]bool{f.bcastRecv: true}
		}
		chs.(map[chan<- *bcast]bool)[f.bcastRecv] = true
		return chs
	}, false)
}

func removeNextHop(name ndn.Name, f *Face) {
	Fib.Update(name, func(chs interface{}) interface{} {
		f.log("remove-nexthop", name)
		delete(f.fibNames, name.String())
		if chs == nil {
			return nil
		}
		m := chs.(map[chan<- *bcast]bool)
		if _, ok := m[f.bcastRecv]; !ok {
			return chs
		}
		delete(m, f.bcastRecv)
		if len(m) == 0 {
			return nil
		}
		return chs
	}, false)
}
