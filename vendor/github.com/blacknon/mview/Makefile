# kernel-style V=1 build verbosity
ifeq ("$(origin V)", "command line")
	BUILD_VERBOSE = $(V)
endif

ifeq ($(BUILD_VERBOSE),1)
	Q =
else
	Q = @
endif

define go_get
	$(Q)command -v $(1) > /dev/null || GO111MODULE=off go get $(2)
endef

export CGO_ENABLED := 1

.DEFAULT_GOAL := help

.PHONY: validate
validate: check-fmt test vet ## Validates the go code format, runs tests and executes vet.

.PHONY: test
test: ## Run tests
	$(Q)echo "running tests..."
	$(Q)go test -race -v ./...

.PHONY: vet
vet: ## Run go vet
	$(Q)echo "running go vet..."
	$(Q)go vet -composites=false ./...

.PHONY: check-fmt
check-fmt: ## Check go format
	$(Q)echo "checking format..."
	@gofmt_out=$$(gofmt -d -e . 2>&1) && [ -z "$${gofmt_out}" ] || (echo "$${gofmt_out}" 1>&2; exit 1)

.PHONY: fmt
fmt: ## Formats the go code
	$(Q)echo "formatting go code..."
	$(Q)set -e
	$(call go_get,goimports,golang.org/x/tools/cmd/goimports)
	$(Q)goimports -w .

.PHONY: help
help: ## Shows this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
