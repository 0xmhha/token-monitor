# Token Monitor - Testing Guide

Comprehensive guide for testing Token Monitor codebase.

---

## Table of Contents

- [Testing Strategy](#testing-strategy)
- [Running Tests](#running-tests)
- [Writing Tests](#writing-tests)
- [Test Coverage](#test-coverage)
- [CI/CD Integration](#cicd-integration)
- [Best Practices](#best-practices)

---

## Testing Strategy

### Testing Pyramid

Token Monitor follows the standard testing pyramid:

```
         /\
        /  \  E2E Tests (~5%)
       /____\
      /      \  Integration Tests (~15%)
     /________\
    /          \  Unit Tests (~80%)
   /__________  \
```

**Unit Tests** (80%)
- Test individual functions and methods in isolation
- Fast execution (<1s per package)
- Mock external dependencies
- Target: >80% code coverage

**Integration Tests** (15%)
- Test interaction between multiple components
- Use real dependencies where possible
- Test complete workflows
- Current coverage: Core workflows tested

**End-to-End Tests** (5%)
- Test complete CLI workflows
- Minimal mocking
- Validate user-facing behavior
- Current: CLI command parsing tests

---

## Running Tests

### Quick Start

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detector
go test -race ./...

# Run specific package tests
go test ./pkg/parser

# Run specific test
go test ./pkg/parser -run TestParseLine

# Run with verbose output
go test -v ./...
```

### Coverage Reports

**Generate Coverage Report**:

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View coverage summary
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html
```

**Coverage by Package**:

```bash
go test -cover ./... 2>&1 | grep coverage:
```

**Current Coverage**:
```
pkg/aggregator   87.0%
pkg/config       86.6%
pkg/discovery    80.0%
pkg/logger       93.3%
pkg/monitor      78.1%
pkg/parser       90.7%  ✅
pkg/reader       71.1%
pkg/session      75.6%
pkg/display      76.9%
pkg/watcher      70.1%
```

### Race Detection

**Run with Race Detector**:

```bash
# All packages
go test -race ./...

# Specific package
go test -race ./pkg/monitor

# With verbose output
go test -race -v ./pkg/monitor
```

**When to Use**:
- Before committing code with goroutines
- When debugging concurrency issues
- In CI/CD pipeline (always enabled)
- When modifying shared state

### Benchmarks

**Run Benchmarks**:

```bash
# Run all benchmarks
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkParseLine ./pkg/parser

# With memory profiling
go test -bench=. -benchmem ./pkg/parser

# Run N times for stability
go test -bench=. -count=5 ./pkg/parser
```

**Benchmark Example**:

```go
func BenchmarkParseLine(b *testing.B) {
    line := `{"timestamp":"2024-01-15T10:30:00Z",...}`
    p := parser.New()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := p.ParseLine(line)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

---

## Writing Tests

### Test File Structure

**Naming Convention**:
- Test files: `*_test.go`
- In same package as code being tested
- Same directory as implementation

**Example Structure**:

```
pkg/parser/
├── parser.go           # Implementation
├── parser_test.go      # Tests
├── types.go            # Types
└── errors.go           # Errors
```

### Unit Test Template

```go
package parser

import (
    "testing"
)

func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {
            name:    "valid input",
            input:   validInput,
            want:    expectedOutput,
            wantErr: false,
        },
        {
            name:    "invalid input",
            input:   invalidInput,
            want:    nil,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionName(tt.input)

            if (err != nil) != tt.wantErr {
                t.Errorf("FunctionName() error = %v, wantErr %v", err, tt.wantErr)
                return
            }

            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("FunctionName() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Table-Driven Tests

**Pattern**: Use for testing multiple scenarios

```go
func TestUsageValidate(t *testing.T) {
    tests := []struct {
        name    string
        usage   Usage
        wantErr bool
    }{
        {
            name: "valid usage",
            usage: Usage{
                InputTokens:  100,
                OutputTokens: 50,
            },
            wantErr: false,
        },
        {
            name: "negative input tokens",
            usage: Usage{
                InputTokens:  -1,
                OutputTokens: 50,
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.usage.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Mocking Dependencies

**Interface-Based Mocking**:

```go
// Mock parser for testing
type mockParser struct {
    parseFunc func(string) (*UsageEntry, error)
}

func (m *mockParser) ParseLine(line string) (*UsageEntry, error) {
    if m.parseFunc != nil {
        return m.parseFunc(line)
    }
    return nil, errors.New("not implemented")
}

func (m *mockParser) ParseFile(path string, offset int64) ([]UsageEntry, int64, error) {
    return nil, 0, errors.New("not implemented")
}

// Use in tests
func TestReader(t *testing.T) {
    mockP := &mockParser{
        parseFunc: func(line string) (*UsageEntry, error) {
            return &UsageEntry{}, nil
        },
    }

    r, err := reader.New(reader.Config{
        Parser: mockP,
        PositionStore: NewMemoryPositionStore(),
    }, logger.Noop())
    // ... test code
}
```

### Testing with Temporary Files

```go
func TestParseFile(t *testing.T) {
    // Create temp directory (auto-cleaned)
    tmpDir := t.TempDir()

    // Create test file
    testFile := filepath.Join(tmpDir, "test.jsonl")
    content := `{"timestamp":"2024-01-01T00:00:00Z",...}`
    if err := os.WriteFile(testFile, []byte(content), 0600); err != nil {
        t.Fatalf("Failed to create test file: %v", err)
    }

    // Run tests
    entries, _, err := parser.ParseFile(testFile, 0)
    // ... assertions
}
```

### Error Testing

**Test Both Success and Error Paths**:

```go
func TestParseError(t *testing.T) {
    tests := []struct {
        name     string
        err      *ParseError
        wantMsg  string
        wantUnwrap error
    }{
        {
            name: "parse error with line number",
            err: &ParseError{
                Line: 42,
                Data: "invalid data",
                Err:  ErrMalformedJSON,
            },
            wantMsg:    "parse error at line 42: invalid data: malformed JSON line",
            wantUnwrap: ErrMalformedJSON,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := tt.err.Error(); got != tt.wantMsg {
                t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
            }

            if unwrapped := tt.err.Unwrap(); unwrapped != tt.wantUnwrap {
                t.Errorf("Unwrap() = %v, want %v", unwrapped, tt.wantUnwrap)
            }
        })
    }
}
```

### Testing Concurrent Code

```go
func TestConcurrentAccess(t *testing.T) {
    agg := aggregator.New(aggregator.Config{})

    // Run multiple goroutines
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            agg.Add(createEntry())
        }()
    }

    wg.Wait()

    stats := agg.Stats()
    if stats.Count != 100 {
        t.Errorf("Expected 100 entries, got %d", stats.Count)
    }
}
```

---

## Test Coverage

### Coverage Requirements

**Minimum Targets**:
- Overall: 70%
- New code: 80%
- Critical paths: 95% (parser, aggregator, session)

**Current Status**:
```
✅ pkg/parser    90.7%  (exceeds target)
✅ pkg/aggregator 87.0%  (exceeds target)
✅ pkg/config    86.6%  (exceeds target)
✅ pkg/logger    93.3%  (exceeds target)
✅ pkg/discovery 80.0%  (meets target)
✅ pkg/monitor   78.1%  (approaching target)
✅ pkg/session   75.6%  (approaching target)
✅ pkg/display   76.9%  (approaching target)
⚠️  pkg/reader   71.1%  (needs improvement)
⚠️  pkg/watcher  70.1%  (needs improvement)
```

### Improving Coverage

**Find Uncovered Code**:

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./pkg/parser

# View coverage details
go tool cover -func=coverage.out

# Find functions below 80%
go tool cover -func=coverage.out | awk '$NF < 80.0'
```

**Focus Areas**:
1. Error handling paths
2. Edge cases and boundary conditions
3. Concurrent access scenarios
4. Resource cleanup (defer statements)

### Coverage Exclusions

**Acceptable Low Coverage**:
- Generated code
- Trivial getters/setters
- Main package (cmd/)
- Example code

**Not Acceptable**:
- Business logic
- Error handling
- Data transformations
- State management

---

## CI/CD Integration

### GitHub Actions Workflow

Token Monitor uses GitHub Actions for automated testing:

**Workflow File**: `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'

      - name: Run tests
        run: go test -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest
```

### Pre-commit Hooks

**Recommended Git Hooks**:

```bash
#!/bin/bash
# .git/hooks/pre-commit

# Run tests
echo "Running tests..."
go test ./...
if [ $? -ne 0 ]; then
    echo "Tests failed. Commit aborted."
    exit 1
fi

# Run linter
echo "Running linter..."
golangci-lint run ./...
if [ $? -ne 0 ]; then
    echo "Linting failed. Commit aborted."
    exit 1
fi

echo "All checks passed!"
```

### CI Requirements

**All Pull Requests Must**:
1. Pass all tests (with race detector)
2. Pass golangci-lint
3. Maintain or improve coverage
4. Build successfully on all platforms

---

## Best Practices

### Test Organization

**DO**:
✅ Use table-driven tests for multiple scenarios
✅ Test both success and error paths
✅ Use descriptive test names
✅ Keep tests focused and independent
✅ Clean up resources (use defer)
✅ Use t.Helper() for test helpers

**DON'T**:
❌ Skip error handling in tests
❌ Use time.Sleep for synchronization
❌ Share state between tests
❌ Test implementation details
❌ Write flaky tests
❌ Ignore race detector warnings

### Test Naming

**Good Test Names**:
```go
TestParseLine_ValidEntry
TestParseLine_EmptyLine_ReturnsError
TestParseLine_MalformedJSON_ReturnsError
TestAggregator_Add_UpdatesStats
TestReader_ReadFromOffset_ReturnsNewEntries
```

**Avoid**:
```go
TestParser1        // Not descriptive
TestError          // Too generic
TestFunction       // Missing scenario
```

### Assertions

**Clear Error Messages**:

```go
// Good
if got != want {
    t.Errorf("ParseLine() returned %d tokens, want %d", got, want)
}

// Better
if got != want {
    t.Errorf("ParseLine(%q) tokens = %d, want %d", input, got, want)
}

// Bad
if got != want {
    t.Error("wrong value")
}
```

### Test Helpers

```go
// Mark as helper to get correct line numbers in failures
func createTestEntry(t *testing.T, tokens int) *parser.UsageEntry {
    t.Helper()

    return &parser.UsageEntry{
        Timestamp: time.Now(),
        SessionID: "test",
        Message: parser.Message{
            Model: "claude-sonnet-4",
            Usage: parser.Usage{
                InputTokens: tokens,
            },
        },
    }
}
```

### Testing with Context

```go
func TestWithTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()

    entries, err := reader.Read(ctx, testFile)
    if err != nil {
        t.Fatalf("Read() error = %v", err)
    }

    // Assertions
}
```

### Cleanup

```go
func TestResourceCleanup(t *testing.T) {
    r, err := reader.New(config, logger)
    if err != nil {
        t.Fatalf("New() error = %v", err)
    }
    defer func() {
        if closeErr := r.Close(); closeErr != nil {
            t.Errorf("Close() error = %v", closeErr)
        }
    }()

    // Test code
}
```

---

## Common Testing Patterns

### Testing File Operations

```go
func TestFileOperation(t *testing.T) {
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.txt")

    // Write test data
    content := []byte("test content")
    if err := os.WriteFile(testFile, content, 0600); err != nil {
        t.Fatalf("Failed to write test file: %v", err)
    }

    // Test
    // ...
}
```

### Testing with Mock Time

```go
// Use a time provider interface
type TimeProvider interface {
    Now() time.Time
}

// Mock implementation
type mockTime struct {
    now time.Time
}

func (m *mockTime) Now() time.Time {
    return m.now
}

// Test
func TestWithMockTime(t *testing.T) {
    fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
    mock := &mockTime{now: fixedTime}

    // Test with fixed time
}
```

### Testing Goroutines

```go
func TestGoroutineCompletion(t *testing.T) {
    done := make(chan struct{})

    go func() {
        defer close(done)
        // Do work
    }()

    select {
    case <-done:
        // Success
    case <-time.After(5 * time.Second):
        t.Fatal("Goroutine did not complete in time")
    }
}
```

---

## Troubleshooting

### Flaky Tests

**Symptoms**: Tests pass sometimes, fail sometimes

**Solutions**:
1. Remove time.Sleep, use channels for synchronization
2. Avoid shared global state
3. Use race detector to find concurrency issues
4. Make tests deterministic (fixed random seeds, mock time)

### Slow Tests

**Symptoms**: Test suite takes too long

**Solutions**:
1. Run tests in parallel: `t.Parallel()`
2. Use mocks instead of real I/O
3. Reduce test data size
4. Profile tests: `go test -cpuprofile=cpu.prof`

### Low Coverage

**Symptoms**: Coverage below target

**Solutions**:
1. Find uncovered code: `go tool cover -html=coverage.out`
2. Add tests for error paths
3. Test edge cases and boundary conditions
4. Add integration tests for complex workflows

---

## Resources

### Documentation

- [Go Testing Package](https://golang.org/pkg/testing/)
- [Table-Driven Tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [Testing Best Practices](https://github.com/golang/go/wiki/TestComments)

### Tools

- [golangci-lint](https://golangci-lint.run/) - Linter aggregator
- [testify](https://github.com/stretchr/testify) - Testing toolkit
- [go-cmp](https://github.com/google/go-cmp) - Deep comparison

### Further Reading

- [CONTRIBUTING.md](../CONTRIBUTING.md) - Development guide
- [API.md](API.md) - API reference
- [ARCHITECTURE.md](ARCHITECTURE.md) - System design

---

## Summary

**Key Takeaways**:

1. Aim for >80% code coverage
2. Use table-driven tests
3. Always test error paths
4. Run with race detector
5. Keep tests fast and focused
6. Write descriptive test names
7. Clean up resources properly
8. Make tests deterministic
9. Use CI/CD for quality gates
10. Continuously improve test quality

**Remember**: Good tests are:
- **Fast**: Run quickly
- **Independent**: No shared state
- **Repeatable**: Same results every time
- **Self-validating**: Pass/fail is clear
- **Timely**: Written with the code
