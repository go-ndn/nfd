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
	log(s.url)
	if s.handleCommand != nil {
		respond := func(resp *ndn.ControlResponse) {
			d := &ndn.Data{Name: i.Name}
			d.Content, _ = tlv.MarshalByte(resp, 101)
			w.SendData(d)
		}
		// command
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
		respond(respOK)

		s.handleCommand(params, f)
	} else {
		// dataset
		timestamp := make([]byte, 8)
		binary.BigEndian.PutUint64(timestamp, uint64(time.Now().UTC().UnixNano()/1000000))
		d := &ndn.Data{Name: ndn.NewName(s.url)}
		d.Name.Components = append(d.Name.Components, timestamp)
		d.Content, _ = tlv.MarshalByte(s.handleDataset(), 128)
		w.SendData(d)
	}
}

func handleLocal() {
	for _, s := range []*service{
		{
			url: "/localhost/nfd/rib/register",
			handleCommand: func(params *ndn.Parameters, f *face) {
				name := params.Name.String()
				f.route[name] = ndn.Route{
					Origin: params.Origin,
					Cost:   params.Cost,
					Flags:  params.Flags,
				}
				nextHop.add(name, f, params.Flags&ndn.FlagChildInherit != 0)
			},
		},
		{
			url: "/localhost/nfd/rib/unregister",
			handleCommand: func(params *ndn.Parameters, f *face) {
				name := params.Name.String()
				delete(f.route, name)
				nextHop.remove(name, f, true)
			},
		},
		{
			url: "/localhost/nfd/rib/list",
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
	} {
		// NOTE: force mux.Handler to be comparable
		nextHop.add(s.url, &struct{ mux.Handler }{mux.Segmentor(1024)(s)}, false)
	}
}
