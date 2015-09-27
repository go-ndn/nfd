# NDN Forwarding Daemon (go-nfd)

This is an alternative implementation of nfd, NDN forwarding daemon.

Each face runs in its own goroutine, and the core serves faces' forwarding requests with channel communication. By using sensible strategies and only one root certificate, go-nfd tries to be as simple as possible and compatible with nfd.

The author is taylorchu (Tai-Lin Chu). This package is released under GPL2 license.

```
Usage of ./nfd:
  -config string
    	config path (default "nfd.json")
  -debug
    	enable logging
```

```
{
	"NDNCertPath": "key/default.ndncert",
	"Listen": [
		{
			"Network": "tcp",
			"Address": ":6363"
		},
		{
			"Network": "udp",
			"Address": ":6363"
		}
	]
}
```

## Install
```
go get -u github.com/go-ndn/nfd
```

## Supported features

- [x] concurrent-friendly
- [x] `rib` commands
- [x] basic forwarding
- [x] content store
- [x] authentication

## Benchmark

Disclaimer: This is just a relative performance comparison between go-nfd and nfd. Caching, logging and signing are all disabled. The whole experiment is conducted many times to get the average. The data packet is a few MB in size.

BurstyForward: N pairs of consumer and producer directly connect to forwarder. An unique interest/data name is assigned to each pair. After all N producers register prefix, the timer starts. The timer stops as soon as all consumers receive data in parallel.

ForwardRTT: 1 consumer and 1 producer directly connect to forwarder. The timer measures the RTT of interests.

```
go-nfd
BenchmarkBurstyForward-8    2000   1638832 ns/op
BenchmarkForwardRTT-8        20000     63589 ns/op

nfd
BenchmarkBurstyForward-8    1000   1815546 ns/op
BenchmarkForwardRTT-8        30000     59355 ns/op
```

