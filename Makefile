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
# The site has no test suite; `npm run build` is its executable correctness
# check — `astro build` renders every page (a doc missing from docs/ or
# missing its frontmatter fails it), then check-no-js fails the build if any
# page ships JavaScript.
test-site:
	cd site && npm run build

lint-site:
	cd site && npm run lint && npm run typecheck
