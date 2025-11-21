# Contributing to Token Monitor

Thank you for your interest in contributing to Token Monitor! This guide will help you get started.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Code Style Guide](#code-style-guide)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Release Process](#release-process)

---

## Code of Conduct

We are committed to providing a welcoming and inspiring community for all. Please be respectful and constructive in all interactions.

---

## Getting Started

### Prerequisites

- **Go 1.22+**: Install from [golang.org](https://golang.org/dl/)
- **Git**: For version control
- **golangci-lint**: For linting (optional but recommended)
  ```bash
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
  ```

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/token-monitor.git
   cd token-monitor
   ```
3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/yourusername/token-monitor.git
   ```

---

## Development Setup

### 1. Install Dependencies

```bash
go mod download
```

### 2. Build the Project

```bash
go build -o token-monitor ./cmd/token-monitor
```

### 3. Run Tests

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...
```

### 4. Run Linting

```bash
golangci-lint run ./...
```

### 5. Test Locally

```bash
# Run the built binary
./token-monitor list
./token-monitor stats
```

---

## Project Structure

```
token-monitor/
├── cmd/
│   └── token-monitor/          # CLI application entry point
│       ├── main.go             # Main function and routing
│       ├── commands.go         # Command implementations
│       ├── session_commands.go # Session management commands
│       └── config_commands.go  # Configuration commands
├── pkg/                        # Public packages
│   ├── aggregator/            # Token aggregation and burn rate
│   ├── config/                # Configuration management
│   ├── discovery/             # Session file discovery
│   ├── display/               # Output formatting
│   ├── logger/                # Structured logging
│   ├── monitor/               # Live monitoring engine
│   ├── parser/                # JSONL parsing
│   ├── reader/                # Incremental file reading
│   ├── session/               # Session metadata storage
│   └── watcher/               # File system watching
├── docs/                       # Documentation
│   ├── ARCHITECTURE.md        # Technical architecture
│   └── todolist.md            # Development roadmap
├── .github/
│   └── workflows/             # CI/CD pipelines
├── .golangci.yml              # Linter configuration
├── .goreleaser.yml            # Release configuration
├── go.mod                     # Go module definition
└── README.md                  # Project overview
```

---

## Development Workflow

### 1. Create a Feature Branch

```bash
git checkout -b feature/my-new-feature
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `test/` - Test additions/improvements

### 2. Make Changes

Follow the [Code Style Guide](#code-style-guide) and [Testing Requirements](#testing-requirements).

### 3. Test Your Changes

```bash
# Format code
go fmt ./...

# Run tests
go test -race ./...

# Run linter
golangci-lint run ./...
```

### 4. Commit Changes

Write clear, descriptive commit messages:

```
Add burn rate calculation to aggregator

- Implement 5-minute sliding window for burn rate
- Add tokens/min and tokens/hour metrics
- Include tests for rate calculation edge cases
```

Commit message format:
- First line: Brief summary (50 chars or less)
- Blank line
- Detailed description with bullet points

### 5. Push and Create Pull Request

```bash
git push origin feature/my-new-feature
```

Then create a Pull Request on GitHub.

---

## Code Style Guide

### Go Conventions

Follow standard Go conventions:
- Use `gofmt` for formatting (automatically applied by `go fmt`)
- Follow [Effective Go](https://golang.org/doc/effective_go)
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

### Project-Specific Guidelines

#### Error Handling

**Always handle errors explicitly:**

```go
// Good
result, err := operation()
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// Bad - never ignore errors
result, _ := operation()
```

#### Logging

**Use structured logging with context:**

```go
// Good
logger.Info("session discovered",
    "session_id", sessionID,
    "path", path,
    "size", fileSize)

// Bad - avoid string formatting in logs
logger.Info(fmt.Sprintf("Discovered session %s at %s", sessionID, path))
```

#### Variable Naming

- Use `camelCase` for local variables
- Use `PascalCase` for exported functions and types
- Use descriptive names, avoid single letters except for:
  - `i`, `j`, `k` for loop indices
  - `n` for counts
  - `err` for errors

```go
// Good
sessionManager := session.New(config, logger)
tokenCount := aggregator.CountTokens()

// Bad
sm := session.New(config, logger)
tc := aggregator.CountTokens()
```

#### Comments

**Document all exported functions and types:**

```go
// SessionManager manages session metadata storage and retrieval.
// It provides CRUD operations for sessions with name indexing
// for fast lookups.
type SessionManager struct {
    // fields...
}

// SetName assigns a friendly name to a session UUID.
// Returns ErrNameConflict if the name is already in use.
func (m *SessionManager) SetName(uuid, name string) error {
    // implementation...
}
```

#### Package Organization

- Each package should have a single, well-defined responsibility
- Keep packages focused and cohesive
- Avoid circular dependencies
- Use internal/ for private packages

---

## Testing Requirements

### Unit Tests

**Every package must have comprehensive unit tests:**

```go
func TestSessionManager_SetName(t *testing.T) {
    tests := []struct {
        name    string
        uuid    string
        newName string
        wantErr bool
    }{
        {
            name:    "valid name assignment",
            uuid:    "abc123",
            newName: "my-session",
            wantErr: false,
        },
        {
            name:    "duplicate name",
            uuid:    "def456",
            newName: "my-session",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

### Test Coverage

- **Minimum requirement**: 70% coverage
- **Target**: 80%+ coverage for new code
- Run coverage report:
  ```bash
  go test -coverprofile=coverage.out ./...
  go tool cover -html=coverage.out
  ```

### Race Detection

All tests must pass with race detector:

```bash
go test -race ./...
```

### Table-Driven Tests

Use table-driven tests for multiple test cases:

```go
func TestParseEntry(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    Entry
        wantErr bool
    }{
        {
            name:    "valid entry",
            input:   `{"session_id":"abc","tokens":100}`,
            want:    Entry{SessionID: "abc", Tokens: 100},
            wantErr: false,
        },
        // more test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseEntry(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseEntry() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("ParseEntry() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Tests

For features that involve multiple packages, write integration tests:

```go
func TestEndToEndMonitoring(t *testing.T) {
    // Set up test environment
    // Create test JSONL files
    // Start monitoring
    // Verify results
}
```

---

## Pull Request Process

### Before Submitting

1. **Update tests**: Add or update tests for your changes
2. **Run all checks**:
   ```bash
   go fmt ./...
   go vet ./...
   golangci-lint run ./...
   go test -race ./...
   ```
3. **Update documentation**: Update README.md, USAGE.md, or other docs if needed
4. **Update CHANGELOG.md**: Add your changes to the Unreleased section

### PR Template

When creating a PR, include:

**Title**: Brief description of changes

**Description**:
```markdown
## Summary
Brief description of what this PR does

## Changes
- Bullet point list of changes
- Be specific and clear

## Testing
How you tested these changes

## Related Issues
Fixes #123
Related to #456
```

### Review Process

1. **Automated checks**: CI must pass (tests, linting)
2. **Code review**: At least one maintainer must approve
3. **Changes requested**: Address all review comments
4. **Final approval**: Maintainer will merge when ready

### After Merge

Your changes will be included in the next release. Thank you for contributing!

---

## Release Process

### Version Numbers

We follow [Semantic Versioning](https://semver.org/):
- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes

### Creating a Release

Releases are automated via GitHub Actions and goreleaser.

1. Update CHANGELOG.md with version and date
2. Create and push a tag:
   ```bash
   git tag -a v0.2.0 -m "Release v0.2.0"
   git push origin v0.2.0
   ```
3. GitHub Actions will:
   - Run all tests
   - Build binaries for all platforms
   - Create GitHub release
   - Attach binaries to release

---

## Development Tips

### Debugging

**Enable debug logging:**
```bash
export TOKEN_MONITOR_LOG_LEVEL=debug
./token-monitor watch
```

**Use delve for debugging:**
```bash
go install github.com/go-delve/delve/cmd/dlv@latest
dlv debug ./cmd/token-monitor -- watch --session test
```

### Testing with Real Data

**Create test JSONL files:**
```bash
mkdir -p ~/.config/claude/projects/test-session
cat > ~/.config/claude/projects/test-session/usage.jsonl << 'EOF'
{"type":"usage","timestamp":"2024-01-15T10:00:00Z","session_id":"test-123","message":{"model":"claude-3-sonnet","usage":{"input_tokens":100,"output_tokens":50}}}
EOF
```

### Performance Profiling

**CPU profiling:**
```bash
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof
```

**Memory profiling:**
```bash
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof
```

---

## Getting Help

- **Questions**: Open a [Discussion](https://github.com/yourusername/token-monitor/discussions)
- **Bugs**: Open an [Issue](https://github.com/yourusername/token-monitor/issues)
- **Features**: Open an [Issue](https://github.com/yourusername/token-monitor/issues) with "enhancement" label

---

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (see LICENSE file).

---

## Recognition

Contributors are recognized in:
- GitHub contributors page
- CHANGELOG.md for significant contributions
- Release notes

Thank you for making Token Monitor better!
