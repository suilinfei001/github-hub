GO ?= go
BIN_DIR ?= bin
SERVER_ADDR ?= :8080
SERVER_ROOT ?= data

ifeq ($(OS),Windows_NT)
    EXE_SUFFIX := .exe
else
    EXE_SUFFIX :=
endif

SERVER_BIN := $(BIN_DIR)/ghh-server$(EXE_SUFFIX)
CLIENT_BIN := $(BIN_DIR)/ghh$(EXE_SUFFIX)

.PHONY: all build build-server build-client run run-server test vet fmt clean

all: build

build: build-server build-client

build-server:
	$(GO) build -o $(SERVER_BIN) ./cmd/ghh-server

build-client:
	$(GO) build -o $(CLIENT_BIN) ./cmd/ghh

run: run-server

run-server: build-server
	$(SERVER_BIN) --addr $(SERVER_ADDR) --root $(SERVER_ROOT)

test:
	$(GO) test ./... -race -cover

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -rf $(BIN_DIR)
