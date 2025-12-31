# Testing Guide

This document describes the testing strategy and how to run tests for ghidra-panel.

## Test Structure

### Go Tests (`srepanel/`)

- **Unit tests**: Test individual functions and modules in isolation
- **Integration tests**: Test database migrations and multi-module interactions
- **Test coverage**: Aim for >80% coverage on critical paths

#### Test Files

- `username/generator_test.go` - Username generation and sanitization
- `passphrase/generator_test.go` - Passphrase generation and security
- `database/passwords_test.go` - User account management
- `database/migrations_test.go` - Database schema migrations
- `csrf/csrf_test.go` - CSRF token generation and validation

### Java Tests (`jaas/`)

- Unit tests for JAAS plugin and Ghidra Server integration
- Located in `jaas/src/test/java/`

## Running Tests

### Quick Start

```bash
# Run all tests
make test

# Run tests with coverage report
make test-coverage

# Run linter
make lint

# Run all checks (tests + lint)
make check
```

### Go Tests (Detailed)

```bash
cd srepanel

# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with race detection
go test -race ./...

# Run specific package
go test ./username

# Run specific test
go test ./username -run TestGeneratePseudonymous

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Java Tests

```bash
cd jaas

# Run all tests
./gradlew test

# Run specific test class
./gradlew test --tests PanelLoginModuleTest

# View test report
# Open: jaas/build/reports/tests/test/index.html
```

## Test Coverage

View coverage reports:

```bash
# Generate and open in browser
make test-browser

# Or manually:
cd srepanel
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Current coverage targets:**
- Username generator: >95%
- Passphrase generator: >90%
- Database layer: >85%
- CSRF tokens: >90%
- Overall: >80%

## Continuous Integration

Tests run automatically on:
- Pull requests to `main`
- Pushes to `main`

See `.github/workflows/test.yml` for CI configuration.

### CI Checks

1. **Go Tests** - All unit and integration tests
2. **Java Tests** - JAAS plugin tests
3. **Go Lint** - Code quality checks (golangci-lint)
4. **Build** - Verify project builds successfully

## Writing Tests

### Test Naming Conventions

```go
// Function: GeneratePseudonymous
func TestGeneratePseudonymous(t *testing.T) { }

// Specific behavior: deterministic output
func TestGeneratePseudonymous_Deterministic(t *testing.T) { }

// Error case
func TestGeneratePseudonymous_InvalidInput(t *testing.T) { }
```

### Table-Driven Tests

Preferred for testing multiple inputs:

```go
func TestSanitizeUsername(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"lowercase", "Alice", "alice"},
        {"special chars", "user@example", "userexample"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := sanitizeUsername(tt.input)
            if result != tt.expected {
                t.Errorf("got %q, want %q", result, tt.expected)
            }
        })
    }
}
```

### Database Tests

Use temporary databases:

```go
func setupTestDB(t *testing.T) (*DB, func()) {
    t.Helper()
    tmpDir := t.TempDir()
    dbPath := filepath.Join(tmpDir, "test.db")
    
    db, err := Open(dbPath)
    if err != nil {
        t.Fatalf("failed to open test database: %v", err)
    }
    
    cleanup := func() {
        db.Close()
    }
    
    return db, cleanup
}

func TestCreateAccount(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()
    
    // Test code...
}
```

## Benchmarks

Run performance benchmarks:

```bash
cd srepanel

# Run all benchmarks
go test -bench=. ./...

# Specific package
go test -bench=. ./passphrase

# With memory allocation stats
go test -bench=. -benchmem ./username

# Compare performance
go test -bench=. ./username > old.txt
# Make changes...
go test -bench=. ./username > new.txt
benchcmp old.txt new.txt  # Requires golang.org/x/tools/cmd/benchcmp
```

## Common Issues

### Tests fail with "database locked"

Multiple tests accessing same database. Use `t.TempDir()` to create isolated databases.

### Race detector warnings

Run with `-race` flag to detect data races:
```bash
go test -race ./...
```

### Coverage not 100%

That's okay! Focus on:
- Critical security paths (password hashing, CSRF)
- Complex logic (username sanitization)
- Error handling

Don't test:
- Trivial getters/setters
- Generated code (`*.pb.go`)

## Manual Testing

For features requiring OAuth/browser interaction:

```bash
# Run dev server
make dev

# Visit http://localhost:8080
# Test OAuth flows manually
# Check audit logs in admin dashboard
```

## Pre-commit Checks

Before committing:

```bash
make check  # Runs tests + linter
```

Install pre-commit hooks (optional):

```bash
pre-commit install
```

## Resources

- Go testing: https://go.dev/doc/tutorial/add-a-test
- Table-driven tests: https://go.dev/wiki/TableDrivenTests
- golangci-lint: https://golangci-lint.run/
- GitHub Actions: https://docs.github.com/en/actions
