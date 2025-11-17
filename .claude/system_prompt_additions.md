# System Prompt Additions for Code Quality

## Code Quality Standards

NEVER write production code that contains:

1. **panic() statements in normal operation paths** - always return error values
2. **memory leaks** - every allocation must have corresponding cleanup
3. **data corruption potential** - all state transitions must preserve data integrity
4. **inconsistent error handling patterns** - establish and follow single pattern

ALWAYS:

1. **Write comprehensive tests BEFORE implementing features**
2. **Include invariant validation in data structures**
3. **Use proper bounds checking for numeric conversions**
4. **Document known bugs immediately and fix them before continuing**
5. **Implement proper separation of concerns**
6. **Use static analysis tools (golangci-lint, go vet) before considering code complete**

## Development Process Guards

### TESTING REQUIREMENTS:
- Write failing tests first, then implement to make them pass
- Never commit code with known bugs - fix them immediately
- Include table-driven tests for data structures
- Test memory usage patterns, not just functionality
- Validate all edge cases and boundary conditions
- Use race detector for all concurrent code

### ARCHITECTURE REQUIREMENTS:
- Explicit error handling - no hidden panics or ignored errors
- Memory safety - all goroutines must be properly cleaned up
- Performance conscious - avoid unnecessary allocations/copies
- API design - consistent patterns across all public interfaces

### REVIEW CHECKPOINTS:

Before marking any code complete, verify:

1. **No compilation warnings**
2. **All tests pass (including race detection tests)**
3. **Memory usage is bounded and predictable**
4. **No data corruption potential in any code path**
5. **Error handling is comprehensive and consistent**
6. **Code is modular and maintainable**
7. **Documentation matches implementation**
8. **Performance benchmarks show acceptable results**
9. **All goroutines have proper shutdown mechanisms**
10. **Context cancellation is properly handled**

## Go-Specific Quality Standards

### ERROR HANDLING:
- Return errors explicitly for all fallible operations
- Define custom error types with context
- Never ignore errors silently
- Use fmt.Errorf with %w for error wrapping
- Provide meaningful error messages with context

```go
// NEVER DO THIS - ignoring errors
result, _ := someOperation()

// DO THIS - explicit error handling
result, err := someOperation()
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

### MEMORY MANAGEMENT:
- Close all resources (files, connections, channels)
- Use defer for cleanup immediately after resource acquisition
- Ensure all goroutines have termination conditions
- Avoid goroutine leaks with proper context cancellation
- Test for memory leaks in long-running scenarios
- Use sync.Pool for frequently allocated objects when appropriate

```go
// DO THIS - proper resource cleanup
func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("failed to open file: %w", err)
    }
    defer f.Close() // Cleanup immediately after acquisition

    // ... process file
    return nil
}
```

### GOROUTINE MANAGEMENT:
- Always provide context for cancellation
- Use sync.WaitGroup to wait for goroutine completion
- Never start goroutines without shutdown mechanism
- Handle context.Done() in all long-running goroutines
- Test concurrent code with -race flag

```go
// DO THIS - proper goroutine management
func (s *Service) Start(ctx context.Context) error {
    var wg sync.WaitGroup

    wg.Add(1)
    go func() {
        defer wg.Done()
        s.worker(ctx)
    }()

    <-ctx.Done()
    wg.Wait() // Wait for all goroutines to finish
    return nil
}

func (s *Service) worker(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return // Proper shutdown
        case <-ticker.C:
            s.doWork()
        }
    }
}
```

### DATA STRUCTURE INVARIANTS:
- Document all invariants in comments
- Implement validation methods
- Test invariant preservation across all operations
- Use mutexes to protect shared state
- Validate state consistency at module boundaries

```go
// DO THIS - document and validate invariants
type SessionStats struct {
    mu sync.RWMutex

    // Invariant: TotalTokens = InputTokens + OutputTokens +
    //            CacheCreationTokens + CacheReadTokens
    InputTokens         int64
    OutputTokens        int64
    CacheCreationTokens int64
    CacheReadTokens     int64
    TotalTokens         int64
}

// validate checks all invariants
func (s *SessionStats) validate() error {
    s.mu.RLock()
    defer s.mu.RUnlock()

    expected := s.InputTokens + s.OutputTokens +
                s.CacheCreationTokens + s.CacheReadTokens
    if s.TotalTokens != expected {
        return fmt.Errorf("invariant violation: total=%d, expected=%d",
            s.TotalTokens, expected)
    }
    return nil
}
```

### MODULE ORGANIZATION:
- Single responsibility per package
- Clear public/private API boundaries (exported vs unexported)
- Comprehensive package documentation
- Logical dependency hierarchy (avoid circular dependencies)
- Use internal/ for private implementation details

## Critical Patterns to Avoid

### DANGEROUS PATTERNS:
```go
// NEVER DO THIS - production panic
panic("This should never happen")

// NEVER DO THIS - unchecked type conversion
id := int32(size) // Can overflow on 64-bit

// NEVER DO THIS - ignoring errors
someOperation()  // Error ignored

// NEVER DO THIS - leaking goroutines
go func() {
    for {
        doWork() // No exit condition
    }
}()

// NEVER DO THIS - unprotected shared state
var counter int // No mutex protection
go func() { counter++ }()
go func() { counter++ }()

// NEVER DO THIS - not closing channels
ch := make(chan int)
go func() {
    ch <- 1
    // Channel never closed - reader may deadlock
}()
```

### PREFERRED PATTERNS:
```go
// DO THIS - proper error handling
func operation() error {
    result, err := riskyOperation()
    if err != nil {
        return fmt.Errorf("risky operation failed: %w", err)
    }

    if err := process(result); err != nil {
        return fmt.Errorf("processing failed: %w", err)
    }

    return nil
}

// DO THIS - safe conversion
func convertSize(size int64) (int32, error) {
    if size > math.MaxInt32 {
        return 0, fmt.Errorf("size %d exceeds int32 maximum", size)
    }
    if size < math.MinInt32 {
        return 0, fmt.Errorf("size %d below int32 minimum", size)
    }
    return int32(size), nil
}

// DO THIS - explicit error handling
if err := someOperation(); err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// DO THIS - proper goroutine lifecycle
func startWorker(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return // Proper exit
            case <-ticker.C:
                doWork()
            }
        }
    }()
}

// DO THIS - protected shared state
type Counter struct {
    mu    sync.Mutex
    value int
}

func (c *Counter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.value++
}

// DO THIS - proper channel management
func producer(ctx context.Context) <-chan int {
    ch := make(chan int)

    go func() {
        defer close(ch) // Always close channels when done

        for i := 0; i < 10; i++ {
            select {
            case <-ctx.Done():
                return
            case ch <- i:
            }
        }
    }()

    return ch
}
```

## Testing Standards

### COMPREHENSIVE TEST COVERAGE:
- Unit tests for all public functions
- Integration tests for complex interactions
- Table-driven tests for data structures
- Stress tests for long-running operations
- Memory leak detection tests (with profiling)
- Race condition tests (with -race flag)
- Edge case and boundary condition tests
- Benchmark tests for performance-critical code

### TEST ORGANIZATION:
```go
package mypackage

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNormalOperation(t *testing.T) {
    // Test typical usage patterns
    result, err := operation()
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}

func TestEdgeCases(t *testing.T) {
    tests := []struct {
        name     string
        input    Input
        expected Output
        wantErr  bool
    }{
        {
            name:     "zero value",
            input:    Input{},
            expected: Output{},
            wantErr:  false,
        },
        {
            name:     "maximum value",
            input:    Input{Value: math.MaxInt64},
            expected: Output{},
            wantErr:  true,
        },
        // ... more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := operation(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}

func TestErrorConditions(t *testing.T) {
    // Test all error paths
    _, err := operationThatFails()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "expected error message")
}

func TestInvariantsPreserved(t *testing.T) {
    // Verify data structure invariants
    ds := NewDataStructure()

    for i := 0; i < 100; i++ {
        require.NoError(t, ds.Insert(i))
        assert.NoError(t, ds.Validate())
    }
}

func TestConcurrency(t *testing.T) {
    // Test with race detector: go test -race
    var wg sync.WaitGroup
    counter := &Counter{}

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            counter.Increment()
        }()
    }

    wg.Wait()
    assert.Equal(t, 100, counter.Value())
}
```

### MEMORY TESTING:
```go
func TestNoMemoryLeaks(t *testing.T) {
    // Run with: go test -memprofile=mem.prof

    var m1, m2 runtime.MemStats
    runtime.GC()
    runtime.ReadMemStats(&m1)

    // Perform operations that should not leak
    for i := 0; i < 1000; i++ {
        ds := NewDataStructure()
        for j := 0; j < 100; j++ {
            ds.Insert(j)
        }
        for j := 0; j < 50; j++ {
            ds.Remove(j)
        }
    }

    runtime.GC()
    runtime.ReadMemStats(&m2)

    // Allow for some variance, but should not grow significantly
    growth := m2.Alloc - m1.Alloc
    assert.Less(t, growth, uint64(1024*1024),
        "Memory grew by %d bytes - potential leak", growth)
}

func TestGoroutineLeaks(t *testing.T) {
    before := runtime.NumGoroutine()

    ctx, cancel := context.WithCancel(context.Background())
    startWorker(ctx)

    time.Sleep(100 * time.Millisecond)
    cancel()
    time.Sleep(100 * time.Millisecond) // Allow goroutines to exit

    after := runtime.NumGoroutine()
    assert.Equal(t, before, after,
        "Goroutine leak detected: before=%d, after=%d", before, after)
}
```

### BENCHMARK TESTS:
```go
func BenchmarkOperation(b *testing.B) {
    ds := NewDataStructure()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        ds.Operation()
    }
}

func BenchmarkParallelOperation(b *testing.B) {
    ds := NewDataStructure()

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            ds.Operation()
        }
    })
}
```

## Documentation Standards

### CODE DOCUMENTATION:
- Document all public APIs with examples
- Explain complex algorithms and data structures
- Document invariants and preconditions
- Include usage examples in doc comments
- Follow godoc conventions

### PACKAGE DOCUMENTATION:
```go
// Package parser provides JSONL parsing functionality for Claude Code
// usage logs. It extracts token usage metrics from JSONL files and
// validates entries for correctness.
//
// The parser is designed to handle malformed lines gracefully by
// logging warnings and skipping invalid entries rather than failing.
//
// Example usage:
//
//     p := parser.New()
//     entries, offset, err := p.ParseFile("/path/to/session.jsonl", 0)
//     if err != nil {
//         log.Fatal(err)
//     }
//     for _, entry := range entries {
//         fmt.Printf("Tokens: %d\n", entry.Message.Usage.InputTokens)
//     }
package parser
```

### FUNCTION DOCUMENTATION:
```go
// Insert adds a key-value pair to the data structure.
//
// If the key already exists, its value is updated and the old value
// is returned. If the key is new, nil is returned.
//
// Parameters:
//   - key: The key to insert (must be comparable)
//   - value: The value to associate with the key
//
// Returns:
//   - The old value if key existed, nil otherwise
//   - An error if the operation fails
//
// Errors:
//   - ErrInvalidKey: if key is nil or violates constraints
//   - ErrCapacityExceeded: if structure is at capacity
//
// Example:
//
//     ds := NewDataStructure()
//     oldVal, err := ds.Insert("key", "value")
//     if err != nil {
//         return err
//     }
//     if oldVal != nil {
//         fmt.Println("Updated existing key")
//     }
//
// Thread-safety: This method is thread-safe.
//
// Complexity: O(log n) average case, O(n) worst case
func (ds *DataStructure) Insert(key, value interface{}) (interface{}, error) {
    // Implementation
}
```

### ERROR DOCUMENTATION:
```go
// Common errors returned by this package
var (
    // ErrInvalidKey is returned when a key violates constraints
    ErrInvalidKey = errors.New("invalid key")

    // ErrNotFound is returned when a key is not found
    ErrNotFound = errors.New("key not found")

    // ErrCapacityExceeded is returned when structure is full
    ErrCapacityExceeded = errors.New("capacity exceeded")
)

// Error is the error type returned by operations in this package.
// It provides additional context about the failure.
type Error struct {
    Op   string // Operation that failed
    Key  string // Key involved (if applicable)
    Err  error  // Underlying error
}

func (e *Error) Error() string {
    if e.Key != "" {
        return fmt.Sprintf("%s: key=%s: %v", e.Op, e.Key, e.Err)
    }
    return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
    return e.Err
}
```

## Static Analysis Requirements

### MANDATORY TOOLS:
Before any code is considered complete, run:

```bash
# Format code
go fmt ./...

# Vet for common issues
go vet ./...

# Run linter (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
golangci-lint run ./...

# Run tests with race detector
go test -race ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run benchmarks
go test -bench=. -benchmem ./...

# Check for memory leaks
go test -memprofile=mem.prof ./...
go tool pprof mem.prof
```

### GOLANGCI-LINT CONFIGURATION:
Create `.golangci.yml` in project root:

```yaml
linters:
  enable:
    - errcheck      # Check for unchecked errors
    - gosimple      # Simplify code
    - govet         # Vet examines Go source code
    - ineffassign   # Detect ineffectual assignments
    - staticcheck   # Static analysis
    - typecheck     # Type checking
    - unused        # Check for unused code
    - gocritic      # Comprehensive Go linter
    - gocyclo       # Cyclomatic complexity
    - godot         # Check comments end in period
    - misspell      # Spell checker
    - prealloc      # Find slice declarations that could be preallocated
    - unconvert     # Unnecessary type conversions
    - unparam       # Unused function parameters
    - gosec         # Security checker

linters-settings:
  gocyclo:
    min-complexity: 15
  errcheck:
    check-blank: true
  govet:
    check-shadowing: true

issues:
  exclude-use-default: false
  max-issues-per-linter: 0
  max-same-issues: 0
```

## Code Review Checklist

Before submitting code for review or marking complete:

- [ ] All tests pass (`go test ./...`)
- [ ] Race detector passes (`go test -race ./...`)
- [ ] No golangci-lint warnings
- [ ] Test coverage >80% for new code
- [ ] All public APIs documented
- [ ] Error handling is comprehensive
- [ ] No goroutine leaks (verified with tests)
- [ ] No memory leaks (verified with profiling)
- [ ] All resources properly closed (files, connections, etc.)
- [ ] Context cancellation properly handled
- [ ] Invariants documented and validated
- [ ] Benchmarks show acceptable performance
- [ ] No TODO or FIXME comments for critical issues
- [ ] Code follows Go conventions (effective go, code review comments)

## Performance Standards

### OPTIMIZATION RULES:
1. **Measure first** - always profile before optimizing
2. **Optimize hot paths** - focus on code that runs frequently
3. **Avoid premature optimization** - clarity before performance
4. **Use appropriate data structures** - map vs slice vs array
5. **Minimize allocations** - reuse objects when possible
6. **Use sync.Pool for frequently allocated objects**
7. **Profile regularly** - CPU, memory, blocking, mutex contention

### PROFILING:
```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof

# Blocking profiling
go test -blockprofile=block.prof -bench=.
go tool pprof block.prof

# Mutex contention profiling
go test -mutexprofile=mutex.prof -bench=.
go tool pprof mutex.prof
```

This system prompt addition establishes clear quality standards, testing requirements, and architectural principles that must be followed for all Go code in this project.
