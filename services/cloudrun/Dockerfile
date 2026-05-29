FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY services/cloudrun ./services/cloudrun
RUN go build -trimpath -ldflags="-s -w" -o /out/cloudrun ./services/cloudrun/cmd/server

FROM alpine:3.20

RUN adduser -D -H -u 10001 dcp
USER dcp
COPY --from=build /out/cloudrun /usr/local/bin/cloudrun
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/cloudrun"]
