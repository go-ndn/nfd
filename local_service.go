package main

import (
	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
	"github.com/go-ndn/tlv"
)

type service struct {
	url           string
	handleCommand func(*ndn.Parameters, *face)
	handleDataset func() interface{}
}

func (s *service) ServeNDN(w mux.Sender, i *ndn.Interest) {
	respond := func(v interface{}, t uint64) {
		d := &ndn.Data{Name: i.Name}
		d.Content, _ = tlv.MarshalByte(v, t)
		w.SendData(d)
	}
	if s.handleCommand != nil {
		// command
		t := uint64(101)
		cmd := new(ndn.Command)
		tlv.Copy(&i.Name, cmd)
		if cmd.Timestamp <= timestamp || key.Verify(cmd, cmd.SignatureValue.SignatureValue) != nil {
			respond(respNotAuthorized, t)
			return
		}
		timestamp = cmd.Timestamp
		params := &cmd.Parameters.Parameters

		var f *face
		if params.FaceID == 0 {
			f = w.(*face)
		} else {
			var ok bool
			f, ok = faces[params.FaceID]
			if !ok {
				respond(respIncorrectParams, t)
				return
			}
		}
		respond(respOK, t)

		s.handleCommand(params, f)
	} else {
		// dataset
		respond(s.handleDataset(), 128)
	}
}

func handleLocal() {
	for _, s := range []*service{
		{
			url: "/localhost/nfd/rib/register",
			handleCommand: func(params *ndn.Parameters, f *face) {
				f.log("rib/register")
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
				f.log("rib/unregister")
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
		nextHop.add(s.url, s, false)
	}
}
