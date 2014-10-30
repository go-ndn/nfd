Nfd
===
This is an alternative implementation of nfd, ndn forwarding daemon.

It is small, and go get-able.

Each face runs in its own thread, and the main thread handles communication between each faces.

When a face receives an interest from remote face (queue), it will send a forward request to the main thread. The main thread will look up centralized rib to know a list of faces to fulfill this request, and distribute this request to them. After theses faces create request promise, this request will be returned to the originating face along via the main thread.

After the promises are returned, the originating face will check for all promises and write out data.

Routing
=======
Nfd uses link state routing. At flood timer, nfd will broadcast LSA interest to all faces, which contains name prefixes and neighbor information. For efficiency, the neighbor information is only covered in LSA by one of the two nodes of an edge. After LSA is received, if it is floodable (not from itself and is newer), it will be inserted into rib and be broadcasted again. 

Note:

1. fib update will not be immediately reflected after rib is changed.

2. Nfd runs dijkstra with min priority queue O(|E| + |V|log|V|).

Install
=======
```
go get github.com/go-ndn/nfd
```

What is supported
=================
- [x] multi-threaded
- [x] some control commands
- [x] forwarding
- [x] content store
- [x] certificate
- [x] routing (link state)
- [ ] ndn dns

License
=======
GPL2

Benchmark
=========
Disclaimer: This is just a relative performance comparsion between go-nfd and nfd. Caching, logging and signing are all disabled. The whole experiment is conducted many times to get the average. The data packet is a few MB in size.

BurstyForward: N pairs of consumer and producer directly connect to forwarder. An unique interest/data name is assigned to each pair. After all N producers register prefix, the timer starts. The timer stops as soon as all consumers receive data in parallel.

ForwardRTT: 1 consumer and 1 producer directly connect to forwarder. The timer measures the RTT of interests.

```
BurstyForward
go-nfd: 37018602 ns/op
nfd: 371395840 ns/op
go-nfd is 10x faster

ForwardRTT
go-nfd: 900541 ns/op
nfd: 6351451 ns/op
go-nfd is 7x faster
```

