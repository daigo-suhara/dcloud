GO_BUILD_CACHE ?= $(CURDIR)/.cache/go-build
GO_MOD_CACHE ?= $(CURDIR)/.cache/go-mod

.PHONY: test build-control-plane build-cloudrun build

test:
	mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE)
	GOCACHE=$(GO_BUILD_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go test ./...

build-control-plane:
	mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE) bin
	GOCACHE=$(GO_BUILD_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o bin/control-plane ./services/control-plane/cmd/server

build-cloudrun:
	mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE) bin
	GOCACHE=$(GO_BUILD_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o bin/cloudrun ./services/cloudrun/cmd/server

build: build-control-plane build-cloudrun
