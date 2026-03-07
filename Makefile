BINARY_NAME := joplin-mcp
BUILD_DIR   := bin
GOBIN       := $(shell go env GOBIN)

.DEFAULT_GOAL := help
.PHONY: build install clean vet test test-unit test-e2e help

build: ## Build the server binary into bin/
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server/

install: build ## Install binary to GOBIN for use as a user-scoped MCP server
	install -d $(GOBIN)
	install $(BUILD_DIR)/$(BINARY_NAME) $(GOBIN)/$(BINARY_NAME)

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

vet: ## Run static analysis
	go vet ./...

test-unit: ## Run unit and integration tests
	go test -v ./internal/...

test-e2e: ## Run end-to-end tests via Docker Compose
	./scripts/run-e2e.sh

test: vet test-unit ## Run vet + unit tests

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
