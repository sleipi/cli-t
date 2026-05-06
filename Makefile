VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o clitest ./cmd/clitest/

test:
	go test ./...

e2e: build
	./clitest test/e2e/

examples: build
	./clitest examples/

all: test e2e examples

.PHONY: build test e2e examples all
