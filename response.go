package main

import (
	"github.com/go-ndn/ndn"
)

var (
	respIncorrectParams = &ndn.CommandResponse{
		StatusCode: 400,
		StatusText: "Incorrect Parameters",
	}
	respNotAuthorized = &ndn.CommandResponse{
		StatusCode: 403,
		StatusText: "Not Authorized",
	}
)
