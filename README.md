Nfd
===
This is an alternative implementation of nfd, ndn forwarding daemon.

It is small (< 300 SLOC), and go get-able.

Each face runs in its own thread, and the main thread handles communication between each faces.

When a face receives an interest from remote face, it will notify the main thread, which will in turn notify other faces. If a face's rib indicates that this interest can be routed, then it will be send out to remote face; otherwise this face simply ignores. Once the data come back, only the original face will receive it because each interest notification keeps track of the original face.

The rib entry of each face is controlled by remote face with signed command.

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

