FROM registry.access.redhat.com/ubi8/go-toolset:1.15.14 AS builder
RUN go version

COPY . /go/src/github.com/miminar/sdimetrics/exporter
WORKDIR /go/src/github.com/miminar/sdimetrics/exporter
ENV GO111MODULE=on
ENV GOPATH=/go
ENV GOCACHE=""
USER root
RUN set -x && \
    go mod init github.com/miminar/sdimetrics/exporter; \
    go mod vendor && \
    go mod verify

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o sdimetrics_exporter .

# Stage 2 (to create a downsized "container executable", ~7MB)

# If you need SSL certificates for HTTPS, replace `FROM SCRATCH` with:
#
#   FROM alpine:3.7
#   RUN apk --no-cache add ca-certificates
#
FROM registry.access.redhat.com/ubi8-minimal
WORKDIR /root/
COPY --from=builder /go/src/github.com/miminar/sdimetrics/exporter/sdimetrics_exporter .

EXPOSE 2112
ENTRYPOINT ["./sdimetrics_exporter"]
