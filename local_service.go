package main

import (
	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
	"github.com/go-ndn/tlv"
)

type commandService func(*ndn.Parameters, *face)

func (s commandService) ServeNDN(w ndn.Sender, i *ndn.Interest) {
	respond := func(resp *ndn.ControlResponse) {
		d := &ndn.Data{Name: i.Name}
		d.Content, _ = tlv.MarshalByte(resp, 101)
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

	s(params, f)
}

type datasetService func() interface{}

func (s datasetService) ServeNDN(w ndn.Sender, i *ndn.Interest) {
	d := &ndn.Data{Name: i.Name}
	d.Content, _ = tlv.MarshalByte(s(), 128)
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
			nextHop.add(name, f)
		}),
		"/rib/unregister": commandService(func(params *ndn.Parameters, f *face) {
			name := params.Name.String()
			delete(f.route, name)
			nextHop.remove(name, f)
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
		// add version number
		if _, ok := h.(datasetService); ok {
			h = mux.Versioner(h)
		}
		// NOTE: mux.Handler must be comparable
		h = &struct{ mux.Handler }{mux.Segmentor(4096)(h)}
		for _, prefix := range []string{"/localhost/nfd", "/localhop/nfd"} {
			nextHop.add(prefix+suffix, h)
		}
	}
}
