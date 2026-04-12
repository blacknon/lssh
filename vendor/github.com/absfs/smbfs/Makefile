.PHONY: help test test-quick test-unit test-full test-integration test-race test-coverage docker-up docker-down docker-logs docker-wait clean bench coverage lint fmt vet all

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# =============================================================================
# Test Targets
# =============================================================================

test: test-unit ## Run all tests (unit only by default)
	@echo "✓ All tests passed"

test-quick: ## Run unit tests without race detector (fastest)
	@echo "Running quick tests..."
	@go test -count=1 ./...

test-unit: ## Run unit tests with race detector
	@echo "Running unit tests..."
	@go test -v -race -count=1 -timeout 5m ./...

test-full: test-unit test-integration ## Run unit and integration tests
	@echo "✓ Full test suite passed"

test-integration: docker-up docker-wait ## Run integration tests against Docker Samba server
	@echo "Running integration tests..."
	@SMB_SERVER=localhost \
	 SMB_SHARE=testshare \
	 SMB_USERNAME=testuser \
	 SMB_PASSWORD=testpass123 \
	 SMB_DOMAIN=TESTGROUP \
	 go test -v -race -count=1 -tags=integration -timeout 10m ./...
	@$(MAKE) docker-down

test-race: ## Run tests with race detector and longer timeout
	@echo "Running race tests..."
	@go test -v -race -count=1 -timeout 10m ./...

test-coverage: ## Run tests with coverage and check threshold
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@echo "Coverage summary:"
	@go tool cover -func=coverage.out | tail -1
	@echo ""
	@coverage=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	threshold=40; \
	echo "Checking coverage threshold (minimum: $$threshold%)..."; \
	if [ $$(echo "$$coverage < $$threshold" | bc -l) -eq 1 ]; then \
		echo "✗ Coverage $$coverage% is below threshold $$threshold%"; \
		exit 1; \
	else \
		echo "✓ Coverage $$coverage% meets threshold $$threshold%"; \
	fi

# =============================================================================
# Docker Targets
# =============================================================================

docker-up: ## Start Docker Samba test server
	@echo "Starting Samba test server..."
	@docker-compose up -d
	@echo "Samba server starting on localhost:445"

docker-down: ## Stop Docker Samba test server
	@echo "Stopping Samba test server..."
	@docker-compose down

docker-logs: ## Show Docker Samba server logs
	@docker-compose logs -f samba

docker-restart: docker-down docker-up docker-wait ## Restart Docker Samba server

docker-wait: ## Wait for Samba server to be ready
	@echo "Waiting for Samba server to be ready..."
	@for i in $$(seq 1 30); do \
		if nc -z localhost 445 2>/dev/null; then \
			echo "✓ Samba server is ready"; \
			exit 0; \
		fi; \
		echo "Waiting... ($$i/30)"; \
		sleep 2; \
	done; \
	echo "✗ Timeout waiting for Samba server"; \
	exit 1

# =============================================================================
# Code Quality Targets
# =============================================================================

coverage: ## Generate test coverage report (HTML)
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"
	@echo ""
	@go tool cover -func=coverage.out | tail -1

lint: ## Run linter
	@echo "Running linter..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	@golangci-lint run ./...
	@echo "✓ Lint passed"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✓ Code formatted"

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...
	@echo "✓ Vet passed"

# =============================================================================
# Benchmarks
# =============================================================================

bench: ## Run performance benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem -count=3 ./...

bench-integration: docker-up docker-wait ## Run integration benchmarks
	@echo "Running integration benchmarks..."
	@SMB_SERVER=localhost \
	 SMB_SHARE=testshare \
	 SMB_USERNAME=testuser \
	 SMB_PASSWORD=testpass123 \
	 SMB_DOMAIN=TESTGROUP \
	 go test -bench=. -benchmem -count=3 -tags=integration ./...
	@$(MAKE) docker-down

# =============================================================================
# Maintenance Targets
# =============================================================================

clean: ## Clean up Docker resources and test cache
	@echo "Cleaning up..."
	@docker-compose down -v 2>/dev/null || true
	@go clean -testcache
	@rm -f coverage.out coverage.html
	@echo "✓ Cleanup complete"

all: fmt vet lint test-coverage ## Run all checks (format, vet, lint, test with coverage)
	@echo "✓ All checks passed"

ci: fmt vet test-coverage ## CI-friendly checks (no lint, faster)
	@echo "✓ CI checks passed"
