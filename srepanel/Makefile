GO?=go

# Enable CGO for SQLite support
export CGO_ENABLED=1

# Detect OS
ifeq ($(OS),Windows_NT)
    BINARY=srepanel.exe
    SHELL=cmd
else
    BINARY=srepanel
endif

.PHONY: build
build: $(BINARY)

srepanel: go.mod go.sum
	$(GO) build -o $@ .

srepanel.exe: go.mod go.sum
	$(GO) build -o $@ .

# Run tests
.PHONY: test
test:
	$(GO) test -v -race ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	$(GO) test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo Coverage report generated: coverage.html

# Run linter (requires golangci-lint)
.PHONY: lint
lint:
ifeq ($(OS),Windows_NT)
	@where golangci-lint >nul 2>nul || (echo golangci-lint not found. Install from https://golangci-lint.run/usage/install/ && exit /b 1)
else
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install from https://golangci-lint.run/usage/install/" && exit 1)
endif
	golangci-lint run --timeout=5m

# Run tests and open coverage in browser
.PHONY: test-browser
test-browser: test-coverage
	@echo Opening coverage report in browser...
ifeq ($(OS),Windows_NT)
	start coverage.html
else
	@which xdg-open > /dev/null && xdg-open coverage.html || open coverage.html
endif

.PHONY: clean
clean:
	-$(GO) clean
ifeq ($(OS),Windows_NT)
	-del /Q srepanel.exe coverage.out coverage.html 2>nul
	-del /Q srepanel 2>nul
else
	-rm -f srepanel coverage.out coverage.html
endif
