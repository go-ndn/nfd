package main

import (
	"bufio"
	"encoding/base64"
	"github.com/taylorchu/exact"
	"github.com/taylorchu/lpm"
	"github.com/taylorchu/ndn"
	"io/ioutil"
	"net"
	"os"
)

type Forwarder struct {
	fib        *lpm.Matcher
	fibNames   map[*Face]map[string]bool
	forwarded  *exact.Matcher
	verifyKey  ndn.Key
	createFace <-chan net.Conn
}

func (this *Forwarder) Run() {
	reqSend := make(chan *req)
	closed := make(chan *Face)
	for {
		select {
		case conn := <-this.createFace:
			ch := make(chan *ndn.Interest)
			f := &Face{
				Face:       ndn.NewFace(conn, ch),
				reqSend:    reqSend,
				reqRecv:    make(chan *req),
				interestIn: ch,
				closed:     closed,
			}
			this.fibNames[f] = make(map[string]bool)
			f.log("face created")
			go f.Run()
		case b := <-reqSend:
			this.handleReq(b)
		case f := <-closed:
			for nextHop := range this.fibNames[f] {
				this.removeNextHop(nextHop, f)
			}
			delete(this.fibNames, f)
			f.log("face removed")
		}
	}
}

func (this *Forwarder) handleReq(b *req) {
	defer close(b.resp)
	c := new(ndn.ControlInterest)
	err := ndn.Copy(b.interest, c)
	if err == nil {
		// command, answer directly
		d := &ndn.Data{
			Name: b.interest.Name,
		}
		d.Content, err = ndn.Marshal(this.handleCommand(&c.Name, b.sender), 101)
		if err != nil {
			return
		}
		ch := make(chan *ndn.Data, 1)
		ch <- d
		b.resp <- ch
		return
	}
	chs := this.fib.Match(b.interest.Name)
	if chs == nil {
		return
	}
	this.forwarded.Update(exact.Key(b.interest.Name.String()+string(b.interest.Nonce)), func(v interface{}) interface{} {
		if v != nil {
			// loop, ignore req
			return v
		}
		for ch := range chs.(map[chan<- *req]bool) {
			resp := make(chan (<-chan *ndn.Data))
			ch <- &req{
				interest: b.interest,
				sender:   b.sender,
				resp:     resp,
			}
			r, ok := <-resp
			if ok {
				b.resp <- r
			}
		}
		return true
	})
}

func (this *Forwarder) decodePrivateKey(file string) (err error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	err = ndn.SignKey.DecodePrivateKey(b)
	return
}

func (this *Forwarder) decodeCertificate(file string) (err error) {
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
	err = this.verifyKey.DecodePublicKey(d.Content)
	return
}

func (this *Forwarder) handleCommand(c *ndn.Command, f *Face) (resp *ndn.ControlResponse) {
	if this.verifyKey.Verify(c, c.SignatureValue.SignatureValue) != nil {
		resp = RespNotAuthorized
		return
	}
	resp = RespOK
	params := c.Parameters.Parameters
	switch c.Module + "." + c.Command {
	case "fib.add-nexthop":
		this.addNextHop(params.Name.String(), f)
	case "fib.remove-nexthop":
		this.removeNextHop(params.Name.String(), f)
	default:
		resp = RespNotSupported
	}
	return
}

func (this *Forwarder) addNextHop(name string, f *Face) {
	this.fib.Update(lpm.Key(name), func(chs interface{}) interface{} {
		f.log("add-nexthop", name)
		this.fibNames[f][name] = true
		if chs == nil {
			return map[chan<- *req]bool{f.reqRecv: true}
		}
		chs.(map[chan<- *req]bool)[f.reqRecv] = true
		return chs
	}, false)
}

func (this *Forwarder) removeNextHop(name string, f *Face) {
	this.fib.Update(lpm.Key(name), func(chs interface{}) interface{} {
		if chs == nil {
			return nil
		}
		f.log("remove-nexthop", name)
		delete(this.fibNames[f], name)
		m := chs.(map[chan<- *req]bool)
		if _, ok := m[f.reqRecv]; !ok {
			return chs
		}
		delete(m, f.reqRecv)
		if len(m) == 0 {
			return nil
		}
		return chs
	}, false)
}
