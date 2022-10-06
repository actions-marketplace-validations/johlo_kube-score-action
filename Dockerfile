FROM golang:1.19.2 AS builder

ENV CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64

WORKDIR /src
COPY . .

RUN go build \
  -ldflags "-s -w -extldflags '-static'" \
  -o /bin/app \
  . \
  && strip /bin/app

RUN echo "nobody:x:65534:65534:Nobody:/:" > /etc_passwd

FROM scratch

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc_passwd /etc/passwd
COPY --from=builder --chown=65534:0 /bin/app /app

USER nobody
ENTRYPOINT ["/app"]