VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X github.com/tripsyapp/cli/internal/cli.Version=$(VERSION) -X github.com/tripsyapp/cli/internal/cli.Commit=$(COMMIT) -X github.com/tripsyapp/cli/internal/cli.Date=$(DATE)

.PHONY: build test fmt vet check install-script-smoke deploy

build:
	go build -ldflags "$(LDFLAGS)" -o bin/tripsy ./cmd/tripsy
	go build -ldflags "$(LDFLAGS)" -o bin/tripsy-mcp ./cmd/tripsy-mcp

deploy:
	flyctl deploy \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg DATE=$(DATE)

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

install-script-smoke:
	bash -n scripts/install.sh

check: fmt vet test install-script-smoke
