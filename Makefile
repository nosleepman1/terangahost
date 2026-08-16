# TerangaHost - Automation Makefile

BINARY_NAME=terangahost

.PHONY: all build clean test run lint deps

all: build

deps:
	go mod download
	go mod tidy

build:
	go build -ldflags="-s -w" -o bin/$(BINARY_NAME) main.go

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(BINARY_NAME).exe main.go

build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(BINARY_NAME)-linux-amd64 main.go

build-mac:
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/$(BINARY_NAME)-darwin-arm64 main.go

test:
	go test -v -race ./...

clean:
	rm -rf bin/
