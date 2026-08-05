.PHONY: build test lint

build:
	go build -ldflags "-X main.version=$$(git describe --tags --always --dirty)" -o bin/tuhdoo ./cmd/tuhdoo

test:
	go test ./...

lint:
	go vet ./...
