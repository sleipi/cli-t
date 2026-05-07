VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o clitest ./cmd/clitest/

test:
	go test ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run --output.text.print-issued-lines=false ./... || true

e2e: build
	./clitest test/e2e/

examples: build
	./clitest examples/

all: test lint e2e examples

.PHONY: build test lint e2e examples all
