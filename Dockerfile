FROM golang:alpine AS build

COPY . /go/src/github.com/go-ndn/nfd

RUN apk --no-cache add git \
  && go get -v github.com/go-ndn/nfd/...

FROM alpine

COPY --from=build /go/bin/nfd /usr/bin/nfd
COPY key/ /etc/nfd/key/
COPY nfd.json /etc/nfd/

WORKDIR /etc/nfd
CMD ["/usr/bin/nfd", "-config", "/etc/nfd/nfd.json"]
