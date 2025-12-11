GO ?= go
BIN_DIR ?= bin
SERVER_ADDR ?= :8080
SERVER_ROOT ?= data
PREFIX ?= /usr/local
DESTDIR ?=

ifeq ($(OS),Windows_NT)
    EXE_SUFFIX := .exe
else
    EXE_SUFFIX :=
endif

SERVER_BIN := $(BIN_DIR)/ghh-server$(EXE_SUFFIX)
CLIENT_BIN := $(BIN_DIR)/ghh$(EXE_SUFFIX)

# Static build flags
STATIC_FLAGS := CGO_ENABLED=0
LDFLAGS := -ldflags '-s -w'

.PHONY: all build build-server build-client build-static build-server-static build-client-static run run-server test vet fmt clean install uninstall

all: build

build: build-server build-client

build-server:
	$(GO) build -o $(SERVER_BIN) ./cmd/ghh-server

build-client:
	$(GO) build -o $(CLIENT_BIN) ./cmd/ghh

build-static: build-server-static build-client-static

build-server-static:
	$(STATIC_FLAGS) $(GO) build -trimpath $(LDFLAGS) -o $(SERVER_BIN) ./cmd/ghh-server

build-client-static:
	$(STATIC_FLAGS) $(GO) build -trimpath $(LDFLAGS) -o $(CLIENT_BIN) ./cmd/ghh

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

install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 $(SERVER_BIN) $(DESTDIR)$(PREFIX)/bin/
	install -m 755 $(CLIENT_BIN) $(DESTDIR)$(PREFIX)/bin/

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/ghh-server$(EXE_SUFFIX)
	rm -f $(DESTDIR)$(PREFIX)/bin/ghh$(EXE_SUFFIX)
