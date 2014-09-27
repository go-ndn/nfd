Nfd
===
This is an alternative implementation of nfd, ndn forwarding daemon.

It is small (< 300 SLOC), and go get-able.

Each face runs in its own thread, and the main thread handles communication between each faces.

When a face receives an interest from remote face (queue), it will send a forward request to the main thread. The main thread will look up centralized rib to know a list of faces to fulfill this request, and distribute this request to them. After theses faces create request promise, this request will be returned to the originating face along via the main thread.

After the promises are returned, the originating face will check for all promises and write out data.

The rib entry is controlled by remote face with signed command.

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
- [ ] routing
- [ ] ndn dns

License
=======
GPL2

