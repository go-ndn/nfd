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
			HandleCommand: func(params *ndn.Parameters, face *Face) interface{} {
				face.log("rib/register")
				face.route[params.Name.String()] = ndn.Route{
					Origin: params.Origin,
					Cost:   params.Cost,
				}
				AddNextHop(params.Name, face.reqRecv)
				return RespOK
			},
		},
		{
			URL: "/localhost/nfd/rib/unregister",
			HandleCommand: func(params *ndn.Parameters, face *Face) interface{} {
				face.log("rib/unregister")
				delete(face.route, params.Name.String())
				RemoveNextHop(params.Name, face.reqRecv)
				return RespOK
			},
		},
		{
			URL: "/localhost/nfd/rib/list",
			HandleDataset: func() interface{} {
				index := make(map[string]int)
				var routes []ndn.RIBEntry
				for face := range Faces {
					faceID := uint64(uintptr(unsafe.Pointer(face)))
					for name, route := range face.route {
						route.FaceID = faceID
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
