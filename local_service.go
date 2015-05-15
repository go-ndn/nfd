package main

import (
	"encoding/binary"
	"time"

	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
	"github.com/go-ndn/tlv"
)

type service struct {
	url           string
	handleCommand func(*ndn.Parameters, *face)
	handleDataset func() interface{}
}

func (s *service) ServeNDN(w ndn.Sender, i *ndn.Interest) {
	if s.handleCommand != nil {
		log("command", s.url)

		respond := func(resp *ndn.ControlResponse) {
			d := &ndn.Data{Name: i.Name}
			d.Content, _ = tlv.MarshalByte(resp, 101)
			w.SendData(d)
		}
		cmd := new(ndn.Command)
		tlv.Copy(&i.Name, cmd)
		if cmd.Timestamp <= timestamp || key.Verify(cmd, cmd.SignatureValue.SignatureValue) != nil {
			respond(respNotAuthorized)
			return
		}
		timestamp = cmd.Timestamp
		params := &cmd.Parameters.Parameters

		var (
			f  *face
			ok bool
		)
		if params.FaceID == 0 {
			for {
				f, ok = w.(*face)
				if ok {
					break
				}
				if h, ok := w.(mux.Hijacker); ok {
					w = h.Hijack()
				} else {
					break
				}
			}
		} else {
			f, ok = faces[params.FaceID]
		}
		if !ok {
			respond(respIncorrectParams)
			return
		}

		respOK := &ndn.ControlResponse{
			StatusCode: 200,
			StatusText: "OK",
			Parameters: *params,
		}
		respOK.Parameters.FaceID = f.id
		respond(respOK)

		s.handleCommand(params, f)
	} else if s.handleDataset != nil {
		log("dataset", s.url)

		timestamp := make([]byte, 8)
		binary.BigEndian.PutUint64(timestamp, uint64(time.Now().UTC().UnixNano()/1000000))
		d := &ndn.Data{Name: i.Name}
		d.Name.Components = append(d.Name.Components, timestamp)
		d.Content, _ = tlv.MarshalByte(s.handleDataset(), 128)
		w.SendData(d)
	} else {
		log("unknown", i.Name)
	}
}

func handleLocal() {
	for _, s := range []*service{
		{
			url: "/rib/register",
			handleCommand: func(params *ndn.Parameters, f *face) {
				name := params.Name.String()
				f.route[name] = ndn.Route{
					Origin: params.Origin,
					Cost:   params.Cost,
					Flags:  params.Flags,
				}
				nextHop.add(name, f)
			},
		},
		{
			url: "/rib/unregister",
			handleCommand: func(params *ndn.Parameters, f *face) {
				name := params.Name.String()
				delete(f.route, name)
				nextHop.remove(name, f)
			},
		},
		{
			url: "/rib/list",
			handleDataset: func() interface{} {
				index := make(map[string]int)
				var routes []ndn.RIBEntry
				for id, f := range faces {
					for name, route := range f.route {
						route.FaceID = id
						if i, ok := index[name]; ok {
							routes[i].Route = append(routes[i].Route, route)
						} else {
							index[name] = len(routes)
							routes = append(routes, ndn.RIBEntry{
								Name:  ndn.NewName(name),
								Route: []ndn.Route{route},
							})
						}
					}
				}
				return routes
			},
		},
		{
		// nfd local service fallback
		},
	} {
		// NOTE: force mux.Handler to be comparable
		h := &struct{ mux.Handler }{mux.Segmentor(4096)(s)}
		for _, prefix := range []string{"/localhost/nfd", "/localhop/nfd"} {
			nextHop.add(prefix+s.url, h)
		}
	}
}
