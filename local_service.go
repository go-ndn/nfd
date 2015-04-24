package main

import (
	"github.com/go-ndn/ndn"
	"github.com/go-ndn/tlv"
)

type service struct {
	url           string
	handleCommand func(*ndn.Parameters, *face)
	handleDataset func() interface{}
}

func (s *service) handle(req *request) {
	if s.handleCommand != nil {
		// command
		t := uint64(101)
		cmd := new(ndn.Command)
		tlv.Copy(&req.interest.Name, cmd)
		if cmd.Timestamp <= timestamp || key.Verify(cmd, cmd.SignatureValue.SignatureValue) != nil {
			respond(req, respNotAuthorized, t)
			return
		}
		timestamp = cmd.Timestamp
		params := &cmd.Parameters.Parameters

		var f *face
		if params.FaceID == 0 {
			f = req.sender
		} else {
			var ok bool
			f, ok = faces[params.FaceID]
			if !ok {
				respond(req, respIncorrectParams, t)
				return
			}
		}
		respond(req, respOK, t)

		s.handleCommand(params, f)
	} else {
		// dataset
		respond(req, s.handleDataset(), 128)
	}
}

func respond(req *request, v interface{}, t uint64) {
	d := &ndn.Data{Name: req.interest.Name}
	d.Content, _ = tlv.MarshalByte(v, t)
	ch := make(chan *ndn.Data, 1)
	ch <- d
	close(ch)
	req.resp <- ch
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
				addNextHop(name, f, params.Flags&ndn.FlagChildInherit != 0)
			},
		},
		{
			url: "/localhost/nfd/rib/unregister",
			handleCommand: func(params *ndn.Parameters, f *face) {
				f.log("rib/unregister")
				name := params.Name.String()
				delete(f.route, name)
				removeNextHop(name, f, true)
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
		addNextHop(s.url, s, false)
	}
}
