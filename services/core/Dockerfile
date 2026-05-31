FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY services/core ./services/core
RUN go build -trimpath -ldflags="-s -w" -o /out/core ./services/core/cmd/server

FROM alpine:3.20

RUN adduser -D -H -u 10001 dcp
USER dcp
COPY --from=build /out/core /usr/local/bin/core
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/core"]
