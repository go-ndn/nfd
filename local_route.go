package main

import (
	"unsafe"

	"github.com/go-ndn/ndn"
)

type Route struct {
	URL           string
	HandleCommand func(*ndn.Parameters, *Face) interface{}
	HandleDataset func() interface{}
}

var (
	localRoute = []Route{
		{
			URL: "/localhost/nfd/rib/register",
			HandleCommand: func(params *ndn.Parameters, f *Face) interface{} {
				f.log("rib/register")
				f.route[params.Name.String()] = ndn.Route{
					Origin: params.Origin,
					Cost:   params.Cost,
				}
				AddNextHop(params.Name, f.reqRecv)
				return RespOK
			},
		},
		{
			URL: "/localhost/nfd/rib/unregister",
			HandleCommand: func(params *ndn.Parameters, f *Face) interface{} {
				f.log("rib/unregister")
				delete(f.route, params.Name.String())
				RemoveNextHop(params.Name, f.reqRecv)
				return RespOK
			},
		},
		{
			URL: "/localhost/nfd/rib/list",
			HandleDataset: func() interface{} {
				index := make(map[string]int)
				var routes []ndn.RibEntry
				for f := range Faces {
					faceId := uint64(uintptr(unsafe.Pointer(f)))
					for name, route := range f.route {
						route.FaceId = faceId
						if i, ok := index[name]; ok {
							routes[i].Route = append(routes[i].Route, route)
						} else {
							index[name] = len(routes)
							routes = append(routes, ndn.RibEntry{
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
