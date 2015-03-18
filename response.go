package main

import (
	"github.com/go-ndn/ndn"
)

var (
	respOK = &ndn.ControlResponse{
		StatusCode: 200,
		StatusText: "OK",
	}
	respIncorrectParams = &ndn.ControlResponse{
		StatusCode: 400,
		StatusText: "Incorrect Parameters",
	}
	respNotAuthorized = &ndn.ControlResponse{
		StatusCode: 403,
		StatusText: "Not Authorized",
	}
)
