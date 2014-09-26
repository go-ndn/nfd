package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var (
	configPath = flag.String("c", "nfd.json", "nfd config file path")
)

var (
	VerifyKey *ndn.Key
	Forwarded = make(map[string]bool) // check for interest loop
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

	create := make(chan net.Conn)

	go func() {
		bcastSend := make(chan *bcast)
		closed := make(chan *Face)
		for {
			select {
			case conn := <-create:
				ch := make(chan *ndn.Interest)
				f := &Face{
					Face:       ndn.NewFace(conn, ch),
					nextHops:   make(map[string]bool),
					bcastSend:  bcastSend,
					bcastRecv:  make(chan *bcast),
					interestIn: ch,
					dataOut:    make(chan *ndn.Data),
					closed:     closed,
				}
				f.log("face created")
				go f.Listen()
			case b := <-bcastSend:
				c := new(ndn.ControlInterest)
				err := ndn.Copy(b.interest, c)
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
				Fib.Update(b.interest.Name, func(chs interface{}) interface{} {
					if chs == nil {
						return nil
					}
					for ch := range chs.(map[chan<- *bcast]bool) {
						ch <- b
					}
					return chs
				}, false)
			case f := <-closed:
				for nextHop := range f.nextHops {
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
					log.Println(err)
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
				log.Println(err)
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
	service := c.Module + "." + c.Command
	f.log("_", service)
	if VerifyKey.Verify(c, c.SignatureValue.SignatureValue) != nil {
		resp = RespNotAuthorized
		return
	}
	resp = RespOK
	params := c.Parameters.Parameters
	switch service {
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
		f.nextHops[name.String()] = true
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
		delete(f.nextHops, name.String())
		if chs == nil {
			return nil
		}
		m := chs.(map[chan<- *bcast]bool)
		delete(m, f.bcastRecv)
		if len(m) == 0 {
			return nil
		}
		return chs
	}, false)
}
