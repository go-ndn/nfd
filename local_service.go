package main

import (
	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
	"github.com/go-ndn/tlv"
)

type commandService func(*ndn.Parameters, *face)

func (s commandService) ServeNDN(w ndn.Sender, i *ndn.Interest) {
	respond := func(resp *ndn.CommandResponse) {
		d := &ndn.Data{Name: i.Name}
		d.Content, _ = tlv.Marshal(resp, 101)
		w.SendData(d)
	}
	cmd := new(ndn.Command)
	tlv.Copy(cmd, &i.Name)
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
		sender := w
		for {
			f, ok = sender.(*face)
			if ok {
				break
			}
			if h, ok := sender.(mux.Hijacker); ok {
				sender = h.Hijack()
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

	s(params, f)

	respOK := &ndn.CommandResponse{
		StatusCode: 200,
		StatusText: "OK",
		Parameters: *params,
	}
	respOK.Parameters.FaceID = f.id
	respond(respOK)
}

type datasetService func() interface{}

func (s datasetService) ServeNDN(w ndn.Sender, i *ndn.Interest) {
	d := &ndn.Data{Name: i.Name}
	d.Content, _ = tlv.Marshal(s(), 128)
	w.SendData(d)
}

func handleLocal() {
	for suffix, h := range map[string]mux.Handler{
		"/rib/register": commandService(func(params *ndn.Parameters, f *face) {
			name := params.Name.String()
			f.route[name] = ndn.Route{
				Origin: params.Origin,
				Cost:   params.Cost,
				Flags:  params.Flags,
			}
			nextHop.add(name, f.id, f, loopChecker, mux.RawCacher(ndn.ContentStore, false))
		}),
		"/rib/unregister": commandService(func(params *ndn.Parameters, f *face) {
			name := params.Name.String()
			delete(f.route, name)
			nextHop.remove(name, f.id)
		}),
		"/rib/list": datasetService(func() interface{} {
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
		}),
	} {
		_, isDatasetService := h.(datasetService)
		for _, prefix := range []string{"/localhost/nfd", "/localhop/nfd"} {
			if isDatasetService {
				nextHop.add(prefix+suffix, newFaceID(), h, mux.Versioner, mux.Segmentor(4096), cacher, mux.Queuer)
			} else {
				nextHop.add(prefix+suffix, newFaceID(), h)
			}
		}
	}
}
