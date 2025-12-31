# Detect OS for binary path
ifeq ($(OS),Windows_NT)
    BIN=srepanel\srepanel.exe
    SHELL=cmd
else
    BIN=srepanel/srepanel
endif

.PHONY: build
build:
	$(MAKE) -C srepanel build

# Initialize test database with migrations (clean slate)
# Uses YAML config
.PHONY: init-db
init-db: build
	-$(BIN) -db test.db -clean
	$(BIN) -db test.db -init -config config.yaml

# Run dev server (preserves existing database - use this for daily development)
.PHONY: dev
dev: build jaas
	$(BIN) -dev -config config.yaml -db test.db

# Clean database and run dev server (fresh start)
.PHONY: dev-clean
dev-clean: init-db jaas
	$(BIN) -dev -config config.yaml -db test.db

# Run dev server with custom config (YAML format)
.PHONY: dev-config
dev-config: build
ifndef CONFIG
	@echo "Usage: make dev-config CONFIG=path/to/config.yaml"
	@exit 1
endif
	-$(BIN) -db test.db -clean
	$(BIN) -db test.db -init -config $(CONFIG)
	$(BIN) -dev -config $(CONFIG) -db test.db

# Create test database with a test user
.PHONY: test-db
test-db: init-db
	$(BIN) set-password -db test.db -user-id 42 -user richard -pass richard

.PHONY: jaas
jaas:
	$(MAKE) -C jaas build

# Legacy target for backwards compatibility
test.db: test-db

.ONESHELL:
.PHONY: integration
integration:
	$(MAKE) -C jaas integration

.PHONY: clean
clean:
	-@$(MAKE) -C jaas clean 2>nul
	-@$(MAKE) -C srepanel clean 2>nul
ifeq ($(OS),Windows_NT)
	-@if exist test.db del /Q test.db 2>nul
else
	-@rm -f test.db
endif

# Show available OAuth providers in config
.PHONY: show-providers
show-providers:
	@echo "OAuth Providers in config.yaml:"
	@grep -A 1 'enabled: true' config.yaml | grep 'type:' | sed 's/.*type: \(.*\)/  - \1/' || echo "  (install grep/sed for this feature)"

# Run Go tests
.PHONY: test
test:
	$(MAKE) -C srepanel test

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	$(MAKE) -C srepanel test-coverage

# Run Go linter
.PHONY: lint
lint:
	$(MAKE) -C srepanel lint

# Run all checks (tests + lint)
.PHONY: check
check: test lint

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  make build          - Build the srepanel binary"
	@echo "  make jaas           - Build JAAS plugin for Ghidra Server"
	@echo "  make test           - Run all tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make lint           - Run code linter"
	@echo "  make check          - Run tests and linter"
	@echo "  make dev            - Build and run dev server (keeps existing database)"
	@echo "  make dev-clean      - Build and run dev server (fresh database - clean slate)"
	@echo "  make dev-config     - Run dev server with custom config (Usage: make dev-config CONFIG=myconfig.yaml)"
	@echo "  make init-db        - Initialize test database with migrations"
	@echo "  make test-db        - Create test database with test user"
	@echo "  make show-providers - Show configured OAuth providers (requires grep/sed)"
	@echo "  make integration    - Run integration tests"
	@echo "  make clean          - Remove built binaries and test database"
