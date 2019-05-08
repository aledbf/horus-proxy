# Build the manager binary
FROM golang:1.12.5 as builder

# Copy in the go src
WORKDIR /go/src/github.com/aledbf/horus-proxy
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/aledbf/horus-proxy/cmd/manager

# Copy the controller-manager into a thin image
FROM openresty/openresty:1.15.8.1rc1-alpine

WORKDIR /

RUN apk add -U \
    bash \
    diffutils \    
    dumb-init \
    && rm -rf /var/cache/apk/*

COPY rootfs/lua-deps.sh /tmp/lua-deps.sh
RUN /tmp/lua-deps.sh

COPY rootfs/etc /etc

COPY --from=builder /go/src/github.com/aledbf/horus-proxy/manager .

ENTRYPOINT ["/usr/bin/dumb-init","--"]
CMD [ "/manager" ]
