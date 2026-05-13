VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X github.com/tripsyapp/cli/internal/cli.Version=$(VERSION) -X github.com/tripsyapp/cli/internal/cli.Commit=$(COMMIT) -X github.com/tripsyapp/cli/internal/cli.Date=$(DATE)
GO_PACKAGES ?= ./...
GO_FORMAT_PATHS ?= ./cmd ./internal

.PHONY: build test fmt fmt-check vet vulncheck security check install-script-smoke deploy

build:
	go build -ldflags "$(LDFLAGS)" -o bin/tripsy ./cmd/tripsy
	go build -ldflags "$(LDFLAGS)" -o bin/tripsy-mcp ./cmd/tripsy-mcp

deploy:
	flyctl deploy \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg DATE=$(DATE)

test:
	go test $(GO_PACKAGES)

fmt:
	go tool gofumpt -w $(GO_FORMAT_PATHS)

fmt-check:
	test -z "$$(go tool gofumpt -l $(GO_FORMAT_PATHS))"

vet:
	go vet $(GO_PACKAGES)

vulncheck:
	go tool govulncheck $(GO_PACKAGES)

security: vulncheck

install-script-smoke:
	bash -n scripts/install.sh

check: fmt-check vet test install-script-smoke
