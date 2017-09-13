package main

import (
	"errors"

	"github.com/go-ndn/mux"
	"github.com/go-ndn/ndn"
	"github.com/go-ndn/tlv"
)

type collector struct {
	ndn.Sender
	*ndn.Data
}

func (c *collector) SendData(d *ndn.Data) error {
	if c.Data == nil {
		c.Data = d
	}
	return nil
}

var (
	errInvalidTimestamp = errors.New("invalid timestamp")
	errNoData           = errors.New("no data")
)

func (c *core) verify(cmd *ndn.Command) error {
	if cmd.Timestamp <= c.timestamp {
		return errInvalidTimestamp
	}
	r := &collector{}
	err := c.verifier.ServeNDN(r, &ndn.Interest{Name: cmd.SignatureInfo.SignatureInfo.KeyLocator.Name})
	if err != nil {
		return err
	}
	if r.Data == nil {
		return errNoData
	}
	key, err := ndn.CertificateFromData(r.Data)
	if err != nil {
		return err
	}
	return key.Verify(cmd, cmd.SignatureValue.SignatureValue)
}

func (c *core) commandService(s func(*ndn.Parameters, *face)) mux.Handler {
	return mux.HandlerFunc(func(w ndn.Sender, i *ndn.Interest) error {
		respond := func(resp *ndn.CommandResponse) error {
			content, err := tlv.Marshal(resp, 101)
			if err != nil {
				return err
			}
			return w.SendData(&ndn.Data{
				Name:    i.Name,
				Content: content,
			})
		}
		cmd := new(ndn.Command)
		tlv.Copy(cmd, &i.Name)
		if c.verify(cmd) != nil {
			return respond(respNotAuthorized)
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
			return respond(respIncorrectParams)
		}

		s(params, f)

		respOK := &ndn.CommandResponse{
			StatusCode: 200,
			StatusText: "OK",
			Parameters: *params,
		}
		respOK.Parameters.FaceID = f.id
		return respond(respOK)
	})
}

func (c *core) datasetService(s func() interface{}) mux.Handler {
	return mux.Queuer(c.localCacher(mux.Segmentor(4096)(mux.Versioner(
		mux.HandlerFunc(func(w ndn.Sender, i *ndn.Interest) error {
			content, err := tlv.Marshal(s(), 128)
			if err != nil {
				return err
			}
			return w.SendData(&ndn.Data{
				Name:    i.Name,
				Content: content,
			})
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
		FaceID: f.id,
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
	for _, f := range c.face {
		for name, route := range f.route {
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
