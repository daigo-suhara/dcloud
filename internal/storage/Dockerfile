FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY internal ./internal
RUN go build -trimpath -ldflags="-s -w" -o /out/storage ./internal/storage/cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates && update-ca-certificates

COPY --from=build /out/storage /usr/local/bin/storage

ENTRYPOINT ["/usr/local/bin/storage"]
