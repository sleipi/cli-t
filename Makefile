build:
	go build -o clit ./cmd/clit/

test:
	go test ./...

e2e: build
	./clit test/e2e/

examples: build
	./clit examples/

all: test e2e examples

.PHONY: build test e2e examples all
