.PHONY: build vet test test-unit test-e2e

build:
	go build ./cmd/server/

vet:
	go vet ./...

test-unit:
	go test -v ./internal/...

test-e2e:
	./scripts/run-e2e.sh

test: vet test-unit
