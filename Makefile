GO_BUILD_CACHE ?= $(CURDIR)/.cache/go-build
GO_MOD_CACHE ?= $(CURDIR)/.cache/go-mod

.PHONY: test build-core build-cloudrun build

test:
	mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE)
	GOCACHE=$(GO_BUILD_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go test ./...

build-core:
	mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE) bin
	GOCACHE=$(GO_BUILD_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o bin/core ./services/core/cmd/server

build-cloudrun:
	mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE) bin
	GOCACHE=$(GO_BUILD_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o bin/cloudrun ./services/cloudrun/cmd/server

build: build-core build-cloudrun
