GO ?= go
BIN_DIR ?= bin
SERVER_BIN := $(BIN_DIR)/ghh-server
CLIENT_BIN := $(BIN_DIR)/ghh

.PHONY: all build build-server build-client test vet fmt

all: build

build: build-server build-client

build-server:
	$(GO) build -o $(SERVER_BIN) ./cmd/ghh-server

build-client:
	$(GO) build -o $(CLIENT_BIN) ./cmd/ghh

test:
	$(GO) test ./... -race -cover

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...
