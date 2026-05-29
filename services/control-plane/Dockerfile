FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY services/control-plane ./services/control-plane
RUN go build -trimpath -ldflags="-s -w" -o /out/control-plane ./services/control-plane/cmd/server

FROM alpine:3.20

RUN adduser -D -H -u 10001 dcp
USER dcp
COPY --from=build /out/control-plane /usr/local/bin/control-plane
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/control-plane"]
