# NDN Forwarding Daemon (NFD)

This is an alternative implementation of nfd, NDN forwarding daemon.

It is small, and go get-able.

The author is taylorchu (Tai-Lin Chu). This package is released under GPL2 license.

# Design

Each face runs in its own goroutine, and the core handles communication between each faces.

When a face receives an interest from remote face, it sends a forward request to the core. The core looks up centralized RIB for a list of faces to fulfill this request, and distributes this request to them. After theses faces create request promises, these promises return to the originating face along the same path. Finally, the originating face waits for promises and writes out data.

# Install
```
go get github.com/go-ndn/nfd
```

# What is supported

- [x] multi-threaded
- [x] some control commands
- [x] forwarding
- [x] content store
- [x] authentication

# Benchmark

Disclaimer: This is just a relative performance comparison between go-nfd and nfd. Caching, logging and signing are all disabled. The whole experiment is conducted many times to get the average. The data packet is a few MB in size.

BurstyForward: N pairs of consumer and producer directly connect to forwarder. An unique interest/data name is assigned to each pair. After all N producers register prefix, the timer starts. The timer stops as soon as all consumers receive data in parallel.

ForwardRTT: 1 consumer and 1 producer directly connect to forwarder. The timer measures the RTT of interests.

```
go-nfd
BenchmarkBurstyForward     100  14440030 ns/op
BenchmarkForwardRTT    5000    385578 ns/op

nfd
BenchmarkBurstyForward	       3	 342563492 ns/op
BenchmarkForwardRTT	     200	   5766092 ns/op
```

