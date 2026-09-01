.PHONY: test lint build run clean install tidy

BINARY := skill-manager
PKG := ./cmd/skill-manager
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/yxc023/skill-manager/internal/cmd.Version=$(VERSION)

test:
	go test -race -cover ./...

cover-html: test
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run --timeout=5m

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

run:
	go run -ldflags "$(LDFLAGS)" $(PKG) --

install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

tidy:
	go mod tidy

clean:
	rm -rf bin/ coverage.out coverage.html