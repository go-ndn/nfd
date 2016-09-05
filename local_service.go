package main

import (
	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
	"github.com/go-ndn/tlv"
)

type collector struct {
	ndn.Sender
	*ndn.Data
}

func (c *collector) SendData(d *ndn.Data) {
	if c.Data == nil {
		c.Data = d
	}
}

func (c *core) verify(cmd *ndn.Command) bool {
	if cmd.Timestamp <= c.timestamp {
		return false
	}
	r := &collector{}
	c.verifier.ServeNDN(r, &ndn.Interest{Name: cmd.SignatureInfo.SignatureInfo.KeyLocator.Name})
	if r.Data == nil {
		return false
	}
	key, err := ndn.CertificateFromData(r.Data)
	if err != nil {
		return false
	}
	if key.Verify(cmd, cmd.SignatureValue.SignatureValue) != nil {
		return false
	}
	return true
}

func (c *core) commandService(s func(*ndn.Parameters, *face)) mux.Handler {
	return mux.HandlerFunc(func(w ndn.Sender, i *ndn.Interest) {
		respond := func(resp *ndn.CommandResponse) {
			d := &ndn.Data{Name: i.Name}
			d.Content, _ = tlv.Marshal(resp, 101)
			w.SendData(d)
		}
		cmd := new(ndn.Command)
		tlv.Copy(cmd, &i.Name)
		if !c.verify(cmd) {
			respond(respNotAuthorized)
			return
		}
		c.timestamp = cmd.Timestamp
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
			f, ok = c.face[params.FaceID]
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
	})
}

func (c *core) datasetService(s func() interface{}) mux.Handler {
	return mux.Queuer(c.localCacher(mux.Segmentor(4096)(mux.Versioner(
		mux.HandlerFunc(func(w ndn.Sender, i *ndn.Interest) {
			d := &ndn.Data{Name: i.Name}
			d.Content, _ = tlv.Marshal(s(), 128)
			w.SendData(d)
		})))))
}

func (c *core) registerService() {
	for suffix, h := range map[string]mux.Handler{
		"/rib/register":   c.commandService(c.registerRIB),
		"/rib/unregister": c.commandService(c.unregisterRIB),
		"/rib/list":       c.datasetService(c.listRIB),
	} {
		for _, prefix := range []string{"/localhost/nfd", "/localhop/nfd"} {
			c.Handle(prefix+suffix, h)
		}
	}
}

func (c *core) registerRIB(params *ndn.Parameters, f *face) {
	f.route[params.Name.String()] = ndn.Route{
		Origin: params.Origin,
		Cost:   params.Cost,
		Flags:  params.Flags,
	}
	c.nextHop.add(params.Name, f.id, f)
}

func (c *core) unregisterRIB(params *ndn.Parameters, f *face) {
	delete(f.route, params.Name.String())
	c.nextHop.remove(params.Name, f.id)
}

func (c *core) listRIB() interface{} {
	index := make(map[string]int)
	var routes []ndn.RIBEntry
	for id, f := range c.face {
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
}
