# TerangaHost - Makefile

BINARY  := terangahost
PKG     := github.com/nosleepman1/terangahost
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG)/cmd.Version=$(VERSION)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build build-all install test lint fmt deps clean

all: lint test build

deps:
	go mod download
	go mod tidy

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) .

build-all:
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; [ "$$os" = windows ] && ext=".exe"; \
		echo "-> bin/$(BINARY)-$$os-$$arch$$ext"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY)-$$os-$$arch$$ext . || exit 1; \
	done

install:
	CGO_ENABLED=0 go install -trimpath -ldflags="$(LDFLAGS)" .

test:
	go test -race -count=1 ./...

lint:
	@test -z "$$(gofmt -l .)" || { echo "Fichiers non formatés :"; gofmt -l .; exit 1; }
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf bin/
