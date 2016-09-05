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

`NDNCertPath` specifies a list of certificates that are only available on local file system. `Rule` is used for authorization; please see [VerifyRule](https://godoc.org/github.com/go-ndn/mux#VerifyRule).

```
{
  "NDNCertPath": ["key/default.ndncert"],
  "Rule": [
    {
      "DataPattern": "^/ndn/guest/alice/1434508942077/KEY/%00%00$",
      "DataSHA256": "e3a64ce49711fdf29e91a08772599384b747fd924b90a7835079146fcb8d915a"
    },
    {
      "DataPattern": ".*",
      "KeyPattern": "^/ndn/guest/alice/1434508942077/KEY/%00%00$"
    }
  ],
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

In this example, `/ndn/guest/alice/1434508942077/KEY/%00%00` is stored in `key/default.ndncert`, and is the trust anchor. We trust any command that is signed by this trust anchor, and by any key that is signed by this trust anchor.

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
BenchmarkBurstyForward-8       	    2000       	    960958 ns/op
BenchmarkForwardRTT-8          	   30000       	     54651 ns/op

nfd
BenchmarkBurstyForward-8    1000   2081785 ns/op
BenchmarkForwardRTT-8        30000     60595 ns/op
```

