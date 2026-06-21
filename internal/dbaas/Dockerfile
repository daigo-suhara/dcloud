FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY internal ./internal
RUN go build -trimpath -ldflags="-s -w" -o /out/dbaas ./internal/dbaas/cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates && update-ca-certificates

COPY --from=build /out/dbaas /usr/local/bin/dbaas

ENTRYPOINT ["/usr/local/bin/dbaas"]
