FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY internal ./internal
RUN go build -trimpath -ldflags="-s -w" -o /out/identity ./internal/identity/cmd/server

FROM alpine:3.20

RUN adduser -D -H -u 10001 dcloud
USER dcloud
COPY --from=build /out/identity /usr/local/bin/identity
EXPOSE 8083
ENTRYPOINT ["/usr/local/bin/identity"]
