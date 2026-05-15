BINARY := cmc
PKG := github.com/loredunk/china-mirror
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG)/cmd/cmc/cli.Version=$(VERSION)

.PHONY: build install test fmt vet sync-mirrors clean

build: sync-mirrors
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/cmc

install: sync-mirrors
	go install -ldflags "$(LDFLAGS)" ./cmd/cmc

test: sync-mirrors
	go test ./... -race

fmt:
	go fmt ./...

vet:
	go vet ./...

# Keep the embedded copy of mirrors.yml in sync with the source of truth.
# go:embed can only read files within the package directory, so we copy.
sync-mirrors:
	cp data/mirrors.yml internal/mirrors/embedded.yml

clean:
	rm -rf bin/
