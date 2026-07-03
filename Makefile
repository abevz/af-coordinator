GO ?= go

.PHONY: fmt build test

fmt:
	gofmt -w cmd internal

build:
	$(GO) build ./...

test:
	$(GO) test ./...
