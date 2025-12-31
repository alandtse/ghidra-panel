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

.PHONY: clean
clean:
	-$(GO) clean
	-del /Q srepanel.exe 2>nul
	-del /Q srepanel 2>nul
