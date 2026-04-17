BINARY := k8s-controller-analyzer
BUILD_DIR := ./bin
TESTDATA_DIR := ./testdata/simple-operator
WRAPPER := ../../scripts/k8s-controller-analyzer.sh
STRICT_FLAGS := --strict-load

.PHONY: build test test-unit test-tool test-wrapper test-wrapper-parity clean run fmt vet deps generate lint all
.DEFAULT_GOAL := help

help: ## Show available Make targets
	@echo "k8s-controller-analyzer Make targets:"
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9_.-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build analyzer binary
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd

test: test-unit test-tool test-wrapper test-wrapper-parity ## Run all test targets

test-unit: ## Run Go unit tests
	go test ./... -v

test-tool: build ## Run analyzer smoke tests on testdata fixture
	@echo "=== Running analyzer against testdata (no --skill) ==="
	$(BUILD_DIR)/$(BINARY) $(TESTDATA_DIR) $(STRICT_FLAGS) --format json | jq '{schema_version, fact_count: (.facts | length), fact_kinds: [.facts[] | .kind] | unique}'
	@echo ""
	@echo "=== Running analyzer against testdata (--skill architecture) ==="
	$(BUILD_DIR)/$(BINARY) $(TESTDATA_DIR) $(STRICT_FLAGS) --skill architecture --format json | jq '{manifest_count: .manifest.count, manifest_hash: .manifest.hash, categories: [.manifest.entries[] | .category] | unique}'
	@echo ""
	@echo "=== Running analyzer against testdata (--skill api) ==="
	$(BUILD_DIR)/$(BINARY) $(TESTDATA_DIR) $(STRICT_FLAGS) --skill api --format json | jq '{manifest_count: .manifest.count, manifest_hash: .manifest.hash, categories: [.manifest.entries[] | .category] | unique}'
	@echo ""
	@echo "=== Running analyzer against testdata (--skill production-readiness) ==="
	$(BUILD_DIR)/$(BINARY) $(TESTDATA_DIR) $(STRICT_FLAGS) --skill production-readiness --format json | jq '{manifest_count: .manifest.count, manifest_hash: .manifest.hash, categories: [.manifest.entries[] | .category] | unique}'

test-wrapper: ## Run wrapper smoke tests (both argument orders)
	@echo "=== Running wrapper smoke tests (path-first / flags-first) ==="
	bash "$(WRAPPER)" "$(TESTDATA_DIR)" -strict-load -rules crd-version-coverage | jq '{rules: .facts[0].rules, kind: .facts[0].kind}'
	bash "$(WRAPPER)" -strict-load -rules crd-version-coverage "$(TESTDATA_DIR)" | jq '{rules: .facts[0].rules, kind: .facts[0].kind}'
	bash "$(WRAPPER)" --strict-load --rules crd-version-coverage "$(TESTDATA_DIR)" | jq '{rules: .facts[0].rules, kind: .facts[0].kind}'
	bash "$(WRAPPER)" --strict-load --skill architecture --format json "$(TESTDATA_DIR)" | jq '{manifest_count: .manifest.count, has_manifest: (.manifest != null)}'

test-wrapper-parity: build ## Compare wrapper output with direct binary output
	@echo "=== Comparing wrapper output with direct binary output ==="
	@tmp_bin=$$(mktemp); \
	tmp_wrap=$$(mktemp); \
	$(BUILD_DIR)/$(BINARY) --rules crd-version-coverage --format json $(STRICT_FLAGS) "$(TESTDATA_DIR)" | jq -S 'del(.extracted_at)' > $$tmp_bin; \
	bash "$(WRAPPER)" --strict-load --rules crd-version-coverage "$(TESTDATA_DIR)" | jq -S 'del(.extracted_at)' > $$tmp_wrap; \
	diff -u $$tmp_bin $$tmp_wrap; \
	rm -f $$tmp_bin $$tmp_wrap

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

run: build ## Build then run analyzer (use ARGS='...')
	$(BUILD_DIR)/$(BINARY) $(ARGS)

fmt: ## Format Go source files
	go fmt ./...

vet: ## Run go vet static checks
	go vet ./...

deps: ## Tidy module dependencies (tool + testdata fixture)
	go mod tidy
	cd $(TESTDATA_DIR) && go mod tidy

generate: ## Regenerate testdata deepcopy objects via controller-gen
	cd $(TESTDATA_DIR) && go tool controller-gen object paths=./api/...

lint: fmt vet ## Run formatting and vet checks

all: deps generate lint test build ## Run full local pipeline
