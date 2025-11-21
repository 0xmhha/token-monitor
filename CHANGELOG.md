# Changelog

All notable changes to Token Monitor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Planned
- Keyboard shortcuts for live monitoring (q, r, ↑/↓, ?)
- Interactive session selection
- Enhanced session export (CSV, JSON)
- TUI dashboard with bubbletea
- Cost calculation integration
- Advanced filtering for session list

---

## [0.1.0] - 2025-11-21

### Added

#### Core Features
- **Live Token Monitoring**: Real-time monitoring of Claude Code token usage
  - Auto-discovery of Claude Code session files
  - Real-time file watching with fsnotify
  - Incremental reading with position tracking
  - Live terminal updates without flickering

- **Token Aggregation**: Comprehensive token usage tracking
  - Per-session statistics with breakdown by token type
  - Input, output, cache creation, and cache read tokens
  - Burn rate calculation (tokens/min, tokens/hour)
  - 5-minute sliding window for accurate rate estimation

- **Billing Block Tracking**: UTC-based 5-hour billing window monitoring
  - Automatic billing block detection
  - Progress tracking within current block
  - Token projection to block limit
  - Block boundary alerts

- **Session Management**: Metadata storage and organization
  - BoltDB-based persistent storage
  - Friendly session naming with UUID mapping
  - Fast lookups by name or UUID
  - Session CRUD operations
  - Automatic session discovery

#### CLI Commands

- **`stats`**: Display aggregated token statistics
  - Group by model, session, date, or hour
  - Show top N sessions by usage
  - Multiple output formats (table, JSON, simple)
  - Filter by session or model

- **`list`**: List all discovered sessions
  - Shows UUID, name, project path, last activity
  - Simple table format

- **`watch`**: Live monitoring with real-time updates
  - Monitor all sessions or specific session
  - Configurable refresh interval (default: 1s)
  - Shows burn rate and billing block status
  - Delta tracking (cumulative and per-update)
  - Table or simple output format
  - History mode (append without clearing screen)

- **`session name`**: Assign friendly names to sessions
  - Auto-creates session if doesn't exist
  - Validates name uniqueness
  - Names usable anywhere UUIDs are accepted

- **`session list`**: List sessions with sorting
  - Sort by name, date, or UUID
  - Show all or only named sessions
  - Table format with metadata

- **`session show`**: Display detailed session information
  - Lookup by name or UUID
  - Shows full session metadata

- **`session delete`**: Remove session metadata
  - Confirmation prompt (skippable with --force)
  - Preserves original JSONL data files

- **`config show`**: Display current configuration
  - YAML or JSON output format
  - Shows configuration source

- **`config path`**: Show configuration file paths
  - Lists search paths in order of precedence
  - Indicates which paths have config files

- **`config reset`**: Reset configuration to defaults
  - Confirmation prompt (skippable with --force)
  - Custom output path support

#### Infrastructure

- **Configuration System**: Flexible multi-source configuration
  - YAML configuration files
  - Environment variable overrides
  - Command-line flag overrides
  - Hierarchical search paths (., ~/.config, /etc)
  - Validation with helpful error messages

- **Logging System**: Structured logging with slog
  - Multiple log levels (debug, info, warn, error)
  - Configurable output (stdout, stderr, file)
  - Context-aware logging with session IDs
  - JSON and text formats

- **CI/CD Pipeline**: Automated testing and building
  - GitHub Actions workflow
  - Test execution with race detector
  - golangci-lint validation
  - Multi-platform builds (Linux, macOS, Windows)
  - Automated releases with goreleaser

- **Code Quality**: Comprehensive quality standards
  - golangci-lint configuration
  - 15+ enabled linters
  - Test file exclusions
  - Security checks (gosec)
  - Performance validation

#### Testing

- **Unit Tests**: Comprehensive test coverage
  - 78%+ coverage for monitor package
  - 87% coverage for aggregator package
  - 86% coverage for config package
  - Race detector enabled
  - Table-driven tests
  - Mock implementations

- **Test Packages**:
  - `pkg/aggregator`: Token aggregation and burn rate tests
  - `pkg/config`: Configuration loading and validation tests
  - `pkg/discovery`: Session file discovery tests
  - `pkg/display`: Output formatting tests
  - `pkg/logger`: Logging tests
  - `pkg/monitor`: Live monitoring tests
  - `pkg/parser`: JSONL parsing tests
  - `pkg/reader`: Incremental reading tests
  - `pkg/session`: Session manager tests
  - `pkg/watcher`: File watching tests

#### Documentation

- **README.md**: Project overview and quick start
  - Installation instructions
  - Usage examples
  - Feature highlights
  - Configuration guide

- **USAGE.md**: Comprehensive usage guide
  - Complete command reference
  - All flags and options documented
  - Common workflows
  - Troubleshooting section
  - FAQ

- **CONTRIBUTING.md**: Development guide
  - Setup instructions
  - Code style guide
  - Testing requirements
  - PR process
  - Release process

- **ARCHITECTURE.md**: Technical architecture
  - System design
  - Component overview
  - Data flows
  - Technology choices

### Technical Details

#### Performance
- Incremental file reading with position tracking
- Efficient file watching with 100ms debouncing
- BoltDB for fast session metadata lookups
- Streaming JSONL parser for large files

#### Security
- Secure file permissions (0750 for directories, 0600 for files)
- Read-only access to Claude data files
- No external network connections
- Local-only data storage

#### Compatibility
- Go 1.22.6 or later
- macOS, Linux, Windows support
- amd64 and arm64 architectures
- Claude Code usage.jsonl format

### Known Limitations

- Session export limited to JSON output (CSV planned)
- No interactive session selection from list
- No keyboard shortcuts in live monitoring
- No TUI dashboard mode
- Cost calculation not yet implemented
- Log rotation not implemented

### Dependencies

- `github.com/fsnotify/fsnotify` v1.7.0 - File system watching
- `go.etcd.io/bbolt` v1.3.10 - Embedded database
- `gopkg.in/yaml.v3` v3.0.1 - YAML configuration
- `github.com/stretchr/testify` v1.8.1 - Testing utilities

---

## Release Notes Format

### Types of Changes

- `Added` - New features
- `Changed` - Changes in existing functionality
- `Deprecated` - Soon-to-be removed features
- `Removed` - Removed features
- `Fixed` - Bug fixes
- `Security` - Security improvements

---

[Unreleased]: https://github.com/yourusername/token-monitor/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/yourusername/token-monitor/releases/tag/v0.1.0
