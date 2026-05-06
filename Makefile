VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o clit ./cmd/clit/

test:
	go test ./...

e2e: build
	./clit test/e2e/

examples: build
	./clit examples/

all: test e2e examples

.PHONY: build test e2e examples all
