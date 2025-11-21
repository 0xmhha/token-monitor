# Token Monitor - TODO List

> **Purpose**: This document tracks remaining tasks for the token-monitor project. Completed features have been moved to archive.

**Project Status**: v0.1.0 released! Core functionality complete and production-ready.

**Last Updated**: 2025-11-21

---

## 🎉 v0.1.0 Release - COMPLETED

### Release Tasks ✅
- [x] Create git tag (v0.1.0)
- [x] Generate release notes (RELEASE_NOTES.md)
- [x] Test goreleaser build locally
- [x] Fix goreleaser deprecation warnings
- [x] Push commits and tags to GitHub
- [x] GitHub release created (https://github.com/0xmhha/token-monitor/releases/tag/v0.1.0)

### Release Artifacts
- Multi-platform binaries (Linux, macOS, Windows)
- Architecture support (amd64, arm64)
- Checksums for verification
- Complete documentation

---

## 🎯 Current Sprint - Post-Release (Priority: Medium)

### Manual Testing (Optional)
- [ ] Manual testing across platforms
  - [ ] macOS testing
  - [ ] Linux testing
  - [ ] Windows testing

---

## 📦 Post-Release Improvements (Priority: Medium)

### CLI Enhancements
- [ ] Add global flags
  - [ ] `--log-level` - Control logging verbosity
  - [ ] `--json` - JSON output for all commands
  - [ ] `--no-color` - Disable colored output

- [ ] Implement `config set <key> <value>`
  - [ ] Update configuration values
  - [ ] Persist to config file
  - [ ] Add validation

- [ ] Add `config validate`
  - [ ] Validate current configuration
  - [ ] Report errors with suggestions

- [ ] Implement `session export <name|uuid>`
  - [ ] Export to CSV format
  - [ ] Export to JSON format
  - [ ] Include all metrics and metadata

- [ ] Enhanced `session show` output
  - [ ] Token breakdown by type (table/chart)
  - [ ] Billing blocks timeline
  - [ ] Activity timeline with timestamps

- [ ] Add `session list` filters
  - [ ] Filter by project path
  - [ ] Filter by date range
  - [ ] Filter by token usage thresholds

### Live Monitoring Enhancements
- [ ] Add keyboard shortcuts
  - [ ] `q` - Quit gracefully
  - [ ] `r` - Reset statistics
  - [ ] `↑/↓` - Navigate between sessions
  - [ ] `?` - Show help overlay

- [ ] Implement interactive session selection
  - [ ] Show numbered list when no session specified
  - [ ] Allow user to select from list

### Session Management
- [ ] Add session metadata fields
  - [ ] Project path indexing for faster lookups
  - [ ] Session tagging system
  - [ ] Custom descriptions

- [ ] Implement database migrations
  - [ ] Version schema
  - [ ] Auto-migration on startup
  - [ ] Backup before migration

- [ ] Add data backup/restore
  - [ ] Export all session metadata
  - [ ] Import from backup file
  - [ ] Scheduled backups

### Installation & Distribution
- [ ] Create Homebrew tap
  - [ ] Formula for macOS/Linux
  - [ ] Automated updates

- [ ] Create Scoop bucket (Windows)
  - [ ] Manifest file
  - [ ] Automated updates

- [ ] Document `go install` method
  - [ ] Installation instructions
  - [ ] Version pinning

- [ ] Direct binary downloads
  - [ ] Documentation for manual install
  - [ ] Verification instructions

---

## 🚀 Feature Enhancements (Priority: Low)

### Advanced Display Features

#### TUI Framework with Bubbletea
- [ ] Implement full-screen dashboard
  - [ ] Real-time token usage with progress bars
  - [ ] Burn rate visualization
  - [ ] Billing block timeline
  - [ ] Multi-pane layout

- [ ] Add display modes
  - [ ] Full dashboard mode
  - [ ] Compact single-line status
  - [ ] Split-screen multi-session view

- [ ] Implement color scheme
  - [ ] Token type colors (input/output/cache)
  - [ ] Status indicators (active/warning/error)
  - [ ] Burn rate thresholds (low/medium/high)

- [ ] Terminal size adaptation
  - [ ] Responsive layout
  - [ ] Automatic fallback for small terminals

### Performance Optimizations

#### Caching & Memory Management
- [ ] Implement LRU cache for session stats
  - [ ] Configurable cache size (default: 100 sessions)
  - [ ] Automatic eviction
  - [ ] Cache hit/miss metrics

- [ ] Add memory pooling
  - [ ] Buffer pools for file reading
  - [ ] Object pools for data structures
  - [ ] Reduce GC pressure

#### Concurrent Processing
- [ ] Worker pool for file processing
  - [ ] Configurable pool size
  - [ ] Job queue with backpressure
  - [ ] Graceful shutdown

- [ ] Parallel file processing
  - [ ] Process multiple sessions concurrently
  - [ ] Coordinate result aggregation

#### I/O Optimization
- [ ] Batched database writes
  - [ ] 100ms write window
  - [ ] Transaction support
  - [ ] Reduced disk I/O

- [ ] File handle connection pooling
  - [ ] Reuse file handles
  - [ ] Resource limits
  - [ ] Automatic cleanup

### Entry Deduplication
- [ ] Hash-based deduplication (`pkg/dedup`)
  - [ ] SHA256 hash of message ID + timestamp
  - [ ] In-memory hash set
  - [ ] 24-hour retention window
  - [ ] Periodic cleanup

### File Watcher Edge Cases
- [ ] Handle file rotation
- [ ] Directory creation/deletion events
- [ ] Symlink support
- [ ] Network filesystem compatibility

### Logging Improvements
- [ ] Implement log rotation
  - [ ] Size-based rotation
  - [ ] Time-based rotation
  - [ ] Configurable retention

---

## 🧪 Testing & Quality (Priority: Medium)

### Integration Tests
- [ ] End-to-end workflow tests
  - [ ] Write JSONL → detect → parse → aggregate → display
  - [ ] Multi-session scenarios
  - [ ] Configuration loading
  - [ ] Error recovery flows

### Performance Tests
- [ ] Benchmark file parsing (target: >10K lines/sec)
- [ ] Stress test with 100+ sessions
- [ ] Memory leak detection (24-hour run)
- [ ] I/O throughput measurement
- [ ] Concurrent access benchmarks

---

## 💡 Future Enhancements

### Cost Analysis
- [ ] Integrate pricing API (LiteLLM)
- [ ] Calculate actual costs per session
- [ ] Budget tracking and alerts
- [ ] Cost projections
- [ ] Generate cost reports

### Web Dashboard
- [ ] HTTP REST API
- [ ] WebSocket for live updates
- [ ] React/Vue frontend
- [ ] Authentication & multi-user support
- [ ] Real-time charts and analytics

### Alerting System
- [ ] Token limit warnings
- [ ] Cost threshold notifications
- [ ] Slack/Discord integration
- [ ] Email notifications
- [ ] Webhook support

### Advanced Analytics
- [ ] Historical trend analysis
- [ ] Usage pattern detection
- [ ] Anomaly detection
- [ ] Forecasting models
- [ ] Custom metrics
- [ ] Prometheus metrics export
- [ ] Grafana dashboard templates

### Export & Reporting
- [ ] CSV export with custom columns
- [ ] PDF report generation
- [ ] Scheduled reports
- [ ] Custom report templates

---

## 📊 Current Project Status

### ✅ Completed Core Features

**Infrastructure**
- ✅ Go project structure with standard layout
- ✅ Configuration system (YAML + env vars + CLI flags)
- ✅ Structured logging with slog
- ✅ CI/CD pipeline (GitHub Actions)
- ✅ golangci-lint configuration
- ✅ goreleaser for multi-platform builds

**Data Layer**
- ✅ JSONL parser with validation
- ✅ Claude data directory discovery
- ✅ Session metadata storage (BoltDB)
- ✅ Session naming and lookup

**Monitoring Engine**
- ✅ File watcher (fsnotify) with debouncing
- ✅ Incremental file reader with position tracking
- ✅ Token aggregator with burn rate calculation
- ✅ Billing block detection (5-hour UTC windows)
- ✅ Live monitor with real-time updates

**CLI Commands**
- ✅ `stats` - Display aggregated statistics
- ✅ `list` - List all discovered sessions
- ✅ `watch` - Live monitoring with real-time updates
- ✅ `session name` - Assign friendly names
- ✅ `session list` - List sessions with sorting
- ✅ `session show` - Show session details
- ✅ `session delete` - Remove session metadata
- ✅ `config show` - Display current configuration
- ✅ `config path` - Show configuration file paths
- ✅ `config reset` - Reset to defaults

**Display**
- ✅ Table output format
- ✅ JSON output format
- ✅ Simple text format
- ✅ Real-time terminal updates (no flickering)
- ✅ Burn rate display (tokens/min, tokens/hour)
- ✅ Billing block progress

**Testing**
- ✅ Comprehensive unit tests (71-90% coverage)
- ✅ Race detector enabled tests
- ✅ Mock implementations for testing
- ✅ CLI command parsing tests with flag validation
- ✅ Parser package: 90.7% coverage
- ✅ Reader package: 71.1% coverage

**Documentation**
- ✅ README.md with installation and usage
- ✅ USAGE.md - Complete command reference with workflows and troubleshooting
- ✅ CONTRIBUTING.md - Development guide with code style and PR process
- ✅ CHANGELOG.md - v0.1.0 release notes with all features
- ✅ ARCHITECTURE.md - Technical architecture with implementation status
- ✅ API.md - Complete API reference for all public packages
- ✅ TESTING.md - Testing guide with strategy and best practices
- ✅ Inline code documentation

### 🎯 Next Milestone: v0.1.0 Release

**Remaining for v0.1.0:**
1. Manual testing on target platforms (optional)
2. Create and publish release

**Estimated Effort**: < 1 day

---

## 📝 Notes

- All core functionality is working and tested
- Code quality verified with golangci-lint
- All tests pass with race detector
- Ready for production use after documentation
- Future enhancements are optional improvements
