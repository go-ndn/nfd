package main

import "github.com/go-ndn/ndn"

type route struct {
	url           string
	handleCommand func(*ndn.Parameters, *face) interface{}
	handleDataset func() interface{}
}

var (
	localRoute = []route{
		{
			url: "/localhost/nfd/rib/register",
			handleCommand: func(params *ndn.Parameters, f *face) interface{} {
				f.log("rib/register")
				f.route[params.Name.String()] = ndn.Route{
					Origin: params.Origin,
					Cost:   params.Cost,
				}
				addNextHop(params.Name, f.reqRecv)
				return respOK
			},
		},
		{
			url: "/localhost/nfd/rib/unregister",
			handleCommand: func(params *ndn.Parameters, f *face) interface{} {
				f.log("rib/unregister")
				delete(f.route, params.Name.String())
				removeNextHop(params.Name, f.reqRecv)
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
	}
)
