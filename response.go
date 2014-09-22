package main

import (
	"github.com/taylorchu/ndn"
)

var (
	RespOK = &ndn.ControlResponse{
		StatusCode: 200,
		StatusText: "OK",
	}
	RespIncorrectParams = &ndn.ControlResponse{
		StatusCode: 400,
		StatusText: "Incorrect Parameters",
	}
	RespNotAuthorized = &ndn.ControlResponse{
		StatusCode: 403,
		StatusText: "Not Authorized",
	}
	RespNotSupported = &ndn.ControlResponse{
		StatusCode: 501,
		StatusText: "Not Supported",
	}
)
