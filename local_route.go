package main

import (
	"github.com/go-ndn/ndn"
	"github.com/go-ndn/tlv"
)

type route struct {
	url           string
	handleCommand func(*ndn.Parameters, *face) *ndn.ControlResponse
	handleDataset func() interface{}
}

func (rt *route) handleReq(rq *req) {
	var (
		v interface{}
		t uint64
	)
	if rt.handleCommand != nil {
		// command
		cmd := new(ndn.Command)
		tlv.Copy(&rq.interest.Name, cmd)
		if cmd.Timestamp <= timestamp || key.Verify(cmd, cmd.SignatureValue.SignatureValue) != nil {
			v = respNotAuthorized
			goto REQ_DONE
		}
		timestamp = cmd.Timestamp
		params := &cmd.Parameters.Parameters

		var f *face
		if params.FaceID == 0 {
			f = rq.sender
		} else {
			var ok bool
			f, ok = faces[params.FaceID]
			if !ok {
				v = respIncorrectParams
				goto REQ_DONE
			}
		}

		t = 101
		v = rt.handleCommand(params, f)
	} else {
		// dataset
		t = 128
		v = rt.handleDataset()
	}

REQ_DONE:
	d := &ndn.Data{Name: rq.interest.Name}
	d.Content, _ = tlv.MarshalByte(v, t)
	ch := make(chan *ndn.Data, 1)
	ch <- d
	close(ch)
	rq.resp <- ch
	close(rq.resp)
}

func handleLocal() {
	for _, rt := range []*route{
		{
			url: "/localhost/nfd/rib/register",
			handleCommand: func(params *ndn.Parameters, f *face) *ndn.ControlResponse {
				f.log("rib/register")
				f.route[params.Name.String()] = ndn.Route{
					Origin: params.Origin,
					Cost:   params.Cost,
				}
				addNextHop(params.Name.String(), f)
				return respOK
			},
		},
		{
			url: "/localhost/nfd/rib/unregister",
			handleCommand: func(params *ndn.Parameters, f *face) *ndn.ControlResponse {
				f.log("rib/unregister")
				delete(f.route, params.Name.String())
				removeNextHop(params.Name.String(), f)
				return respOK
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
		addNextHop(rt.url, rt)
	}
}
