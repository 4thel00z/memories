VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.PHONY: build install test lint clean all install-hooks hooks

all: build

build:
	go build $(LDFLAGS) -o bin/mem ./cmd/mem

build-all: build-linux build-darwin build-windows

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/mem-linux-amd64 ./cmd/mem
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/mem-linux-arm64 ./cmd/mem

build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/mem-darwin-amd64 ./cmd/mem
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/mem-darwin-arm64 ./cmd/mem

build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/mem-windows-amd64.exe ./cmd/mem

install:
	go install $(LDFLAGS) ./cmd/mem

test:
	go test -v ./...

test-cover:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run

fmt:
	go fmt ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

install-hooks:
	go install github.com/evilmartians/lefthook@latest
	lefthook install

hooks: install-hooks
