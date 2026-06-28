BINARY := sift
VERSION := 0.1.0
PREFIX ?= /usr/local

.PHONY: all build install uninstall test clean cross

all: build

## build: compile the sift binary for the current platform
build:
	go build -ldflags "-s -w" -o $(BINARY) .

## install: build and copy the binary into $(PREFIX)/bin
install: build
	install -d $(PREFIX)/bin
	install -m 0755 $(BINARY) $(PREFIX)/bin/$(BINARY)

## uninstall: remove the installed binary
uninstall:
	rm -f $(PREFIX)/bin/$(BINARY)

## test: run the test suite
test:
	go test ./...

## clean: remove build artifacts
clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist

## cross: build release binaries for the major platforms into dist/
cross:
	GOOS=linux   GOARCH=amd64 go build -ldflags "-s -w" -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags "-s -w" -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -ldflags "-s -w" -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags "-s -w" -o dist/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o dist/$(BINARY)-windows-amd64.exe .
