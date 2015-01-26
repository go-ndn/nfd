package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-ndn/exact"
	"github.com/go-ndn/lpm"
	"github.com/go-ndn/ndn"
	"github.com/go-ndn/nfd/uuid"
)

var (
	Id    = uuid.New()
	Faces = make(map[*Face]struct{})

	FaceCreate = make(chan *connReq)
	ReqSend    = make(chan *req)
	FaceClose  = make(chan *Face)

	Forwarded = exact.New()
	Fib       = lpm.New()

	Rib        = make(map[string]*ndn.LSA)
	RibUpdated = false
)

func log(i ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf("[core] %s", fmt.Sprintln(i...))
}

type connReq struct {
	conn net.Conn
	cost uint64
}

func Run() {
	log("start")
	var (
		lsaFloodTimer, lsaExpireTimer, fibUpdateTimer <-chan time.Time
	)
	if !*dummy {
		lsaFloodTimer = time.Tick(LSAFloodIntv)
		lsaExpireTimer = time.Tick(LSAExpireIntv)
		fibUpdateTimer = time.Tick(FibUpdateIntv)
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case b := <-FaceCreate:
			CreateFace(b)
		case b := <-ReqSend:
			HandleReq(b)
		case <-fibUpdateTimer:
			if !RibUpdated {
				continue
			}
			log("update fib")
			RibUpdated = false
			UpdateFib()
		case <-lsaFloodTimer:
			log("flood lsa")
			FloodLSA(CreateLSA(), nil)
		case <-lsaExpireTimer:
			log("remove expired lsa")
			RemoveExpiredLSA()
		case f := <-FaceClose:
			delete(Faces, f)
			for name := range f.registered {
				RemoveNextHop(name, f)
			}
			f.log("face removed")
		case <-quit:
			log("goodbye")
			return
		}
	}
}

func CreateFace(b *connReq) {
	ch := make(chan *ndn.Interest)
	f := &Face{
		Face:         ndn.NewFace(b.conn, ch),
		reqRecv:      make(chan *req),
		interestRecv: ch,
		registered:   make(map[string]bool),
		cost:         b.cost,
	}
	Faces[f] = struct{}{}
	f.log("face created")
	go f.Run()
}

func HandleReq(b *req) {
	defer close(b.resp)
	if strings.HasPrefix(b.interest.Name.String(), "/localhost/nfd/") {
		HandleLocal(b)
		return
	}
	chs := Fib.Match(b.interest.Name)
	if chs == nil {
		return
	}
	k := exact.Key(fmt.Sprintf("%s/%x", b.interest.Name, b.interest.Nonce))
	Forwarded.Update(k, func(v interface{}) interface{} {
		if v != nil {
			b.sender.log("loop detected", k)
			return v
		}
		for ch := range chs.(map[chan<- *req]struct{}) {
			resp := make(chan (<-chan *ndn.Data))
			ch <- &req{
				interest: b.interest,
				sender:   b.sender,
				resp:     resp,
			}
			r, ok := <-resp
			if ok {
				b.resp <- r
				break
			}
		}
		go func() {
			time.Sleep(LoopDetectIntv)
			Forwarded.Remove(k)
		}()
		return struct{}{}
	})
}

func HandleLocal(b *req) {
	c := new(ndn.Command)
	err := ndn.Copy(&b.interest.Name, c)
	if err != nil {
		return
	}
	d := &ndn.Data{Name: b.interest.Name}
	d.Content, err = ndn.Marshal(HandleCommand(c, b.sender), 101)
	if err != nil {
		return
	}
	ch := make(chan *ndn.Data, 1)
	ch <- d
	close(ch)
	b.resp <- ch
}

func HandleCommand(c *ndn.Command, f *Face) (resp *ndn.ControlResponse) {
	if c.Timestamp <= Timestamp || VerifyKey.Verify(c, c.SignatureValue.SignatureValue) != nil {
		resp = RespNotAuthorized
		return
	}
	Timestamp = c.Timestamp
	resp = RespOK
	params := c.Parameters.Parameters
	switch c.Module + "/" + c.Command {
	case "rib/register":
		AddNextHop(params.Name.String(), f, true)
		if *dummy {
			SendControl(c.Module, c.Command, &c.Parameters.Parameters, func(f *Face) bool { return f.cost != 0 })
		}
	case "rib/unregister":
		RemoveNextHop(params.Name.String(), f)
		if *dummy {
			SendControl(c.Module, c.Command, &c.Parameters.Parameters, func(f *Face) bool { return f.cost != 0 })
		}
	case "lsa/flood":
		if *dummy || !IsLSANewer(params.LSA) {
			return
		}
		f.log("flood lsa", params.LSA.Id, "from", params.Uri)
		Rib[params.LSA.Id] = params.LSA
		RibUpdated = true
		f.id = params.Uri
		FloodLSA(params.LSA, f)
	default:
		resp = RespNotSupported
	}
	return
}

func SendControl(module, command string, params *ndn.Parameters, validate func(*Face) bool) {
	c := new(ndn.Command)
	c.Module = module
	c.Command = command
	c.Parameters.Parameters = *params
	i := new(ndn.Interest)
	ndn.Copy(c, &i.Name)
	for f := range Faces {
		if !validate(f) {
			continue
		}
		resp := make(chan (<-chan *ndn.Data))
		f.reqRecv <- &req{
			interest: i,
			resp:     resp,
		}
		<-resp
	}
}
