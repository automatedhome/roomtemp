FROM arm32v7/golang:stretch

COPY qemu-arm-static /usr/bin/
WORKDIR /go/src/github.com/automatedhome/roomtemp
COPY . .
RUN make build

FROM arm32v7/busybox:1.30-glibc

COPY --from=0 /go/src/github.com/automatedhome/roomtemp/roomtemp /usr/bin/roomtemp
COPY --from=0 /go/src/github.com/automatedhome/roomtemp/config.yaml /config.yaml

ENTRYPOINT [ "/usr/bin/roomtemp" ]
