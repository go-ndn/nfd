package main

var config struct {
	Listen []struct {
		Network, Address string
	}
	PrivateKeyPath string
}
