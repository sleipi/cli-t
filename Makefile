VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o clitest ./cmd/clitest/

test:
	go test ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run --output.text.print-issued-lines=false ./... || true

e2e: build helpers
	./clitest --silent test/e2e/

helpers:
	go build -o test/_helpers/bgserver/bgserver ./test/_helpers/bgserver/

examples: build
	./clitest --silent examples/

test-dash:
	podman run --rm -v "$(CURDIR):/work:Z" -w /work golang:1.26-bookworm sh -c "make test e2e examples"

test-ash:
	podman run --rm -v "$(CURDIR):/work:Z" -w /work golang:1.26-alpine sh -c "apk add --no-cache make >/dev/null 2>&1 && make test e2e examples"

all: test lint e2e examples

.PHONY: build test lint e2e examples all helpers test-dash test-ash
