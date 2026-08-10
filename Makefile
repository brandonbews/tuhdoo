.PHONY: build test lint test-go lint-go test-site lint-site

build:
	go build -ldflags "-X main.version=$$(git describe --tags --always --dirty)" -o bin/tuhdoo ./cmd/tuhdoo

test: test-go test-site

lint: lint-go lint-site

test-go:
	go test ./...

lint-go:
	go vet ./...

# Site targets need dependencies installed first (cd site && npm ci).
# The site has no test suite; `next build` is its executable correctness
# check — it compiles, type-checks the routes, and prerenders every page.
test-site:
	cd site && npm run build

lint-site:
	cd site && npm run lint && npm run typecheck
