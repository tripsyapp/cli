.PHONY: build test fmt vet check

build:
	go build -o bin/tripsy ./cmd/tripsy

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

check: fmt vet test
