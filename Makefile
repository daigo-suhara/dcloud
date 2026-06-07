GO_BUILD_CACHE ?= $(CURDIR)/.cache/go-build
GO_MOD_CACHE ?= $(shell go env GOMODCACHE)
GOPATH_BIN := $(shell go env GOPATH)/bin

.PHONY: test build-project build-container build-identity build sqlc proto buf-lint

test:
	mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE)
	GOCACHE=$(GO_BUILD_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go test ./...

sqlc:
	cd internal/db/sqlc && sqlc generate

proto:
	mkdir -p internal/pb api/generated
	PATH=$(GOPATH_BIN):$$PATH protoc -I protos --go_out=. --go_opt=module=github.com/daigo-suhara/dcloud --go-grpc_out=. --go-grpc_opt=module=github.com/daigo-suhara/dcloud protos/*.proto
	protoc -I protos --python_out=api/generated protos/*.proto

buf-lint:
	BUF_CACHE_DIR=$(BUF_CACHE_DIR) buf lint

build-project:
	mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE) bin
	GOCACHE=$(GO_BUILD_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o bin/project ./internal/project/cmd/server

build-container:
	mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE) bin
	GOCACHE=$(GO_BUILD_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o bin/container ./internal/container/cmd/server

build-identity:
	mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE) bin
	GOCACHE=$(GO_BUILD_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o bin/identity ./internal/identity/cmd/server

build: build-project build-container build-identity
