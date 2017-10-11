package main

import (
	"github.com/go-ndn/ndn"
	"github.com/sirupsen/logrus"
)

type face struct {
	ndn.Face
	logrus.FieldLogger
	id    uint64
	route map[string]ndn.Route
}

func (f *face) ServeNDN(w ndn.Sender, i *ndn.Interest) error {
	f.WithField("name", i.Name).Debug("forward")
	d, err := f.SendInterest(i)
	if err != nil {
		return err
	}
	f.WithField("name", d.Name).Debug("receive")
	return w.SendData(d)
}
