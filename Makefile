.PHONY: build test lint

build:
	go build -o bin/tuhdoo ./cmd/tuhdoo

test:
	go test ./...

lint:
	go vet ./...
