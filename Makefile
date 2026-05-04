build:
	go build -o clit ./cmd/clit/

test:
	go test ./...

e2e: build
	./clit examples/

self-test: build
	./clit examples/99_self_test.clit

all: test e2e

.PHONY: build test e2e self-test all
