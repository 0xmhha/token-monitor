# Token Monitor Development TODO List

> **Purpose**: This document tracks all features and tasks for the token-monitor project. It enables seamless continuation across multiple development sessions.

**Project Goal**: Build a real-time token monitoring tool for Claude Code CLI sessions in Go, with session naming, live updates, and persistent tracking.

---

## Phase 1: Foundation & Core Infrastructure

### 1.1 Project Setup ✅ (Partially Complete)
- [x] Create project directory structure
- [x] Initialize .gitignore
- [x] Create docs/ARCHITECTURE.md
- [x] Create docs/todolist.md
- [ ] Wait for user to create .claude folder with development rules
- [ ] Initialize Go module (`go mod init github.com/yourusername/token-monitor`)
- [ ] Set up basic project structure:
  - [ ] `cmd/token-monitor/main.go` - CLI entry point
  - [ ] `pkg/` - Public packages
  - [ ] `internal/` - Private packages
  - [ ] `test/` - Integration tests
- [ ] Create README.md with project overview
- [ ] Set up CI/CD pipeline (GitHub Actions)
  - [ ] Go test runner
  - [ ] Linting (golangci-lint)
  - [ ] Build for multiple platforms

### 1.2 Configuration System
- [ ] Design configuration schema (YAML format)
- [ ] Implement config loader
  - [ ] Search for config in: `./`, `~/.config/token-monitor/`, `/etc/token-monitor/`
  - [ ] Support environment variable overrides
  - [ ] Validate configuration schema
- [ ] Create default configuration file
- [ ] Implement config merge logic (defaults → file → env vars → CLI flags)
- [ ] Add config validation with helpful error messages
- [ ] Write tests for config loading

### 1.3 Logging System
- [ ] Choose logging library (e.g., `zap` or `logrus`)
- [ ] Implement structured logging
- [ ] Add log levels (debug, info, warn, error)
- [ ] Configure log output destinations (stdout, file)
- [ ] Add context-aware logging (request IDs, session IDs)
- [ ] Implement log rotation
- [ ] Write logging utilities and helpers

---

## Phase 2: Data Layer & Parsing

### 2.1 JSONL Parser (`pkg/parser`)
- [ ] Define `UsageEntry` data structures
  - [ ] `UsageEntry` struct
  - [ ] `Message` struct
  - [ ] `TokenUsage` struct
  - [ ] `Content` struct
- [ ] Implement JSONL line parser
  - [ ] JSON unmarshaling with validation
  - [ ] Handle malformed lines gracefully
  - [ ] Extract token usage metrics
- [ ] Add validation logic
  - [ ] Required field checks
  - [ ] Type validation
  - [ ] Range validation (non-negative tokens)
- [ ] Implement streaming parser for large files
  - [ ] Use `bufio.Scanner` for line-by-line reading
  - [ ] Handle file encoding (UTF-8)
  - [ ] Support incremental parsing
- [ ] Write comprehensive parser tests
  - [ ] Valid JSONL entries
  - [ ] Malformed JSON
  - [ ] Missing fields
  - [ ] Edge cases (empty files, huge files)

### 2.2 Data Discovery (`pkg/discovery`)
- [ ] Implement Claude data directory discovery
  - [ ] Check `~/.config/claude/projects/`
  - [ ] Check `~/.claude/projects/` (legacy)
  - [ ] Support `CLAUDE_CONFIG_DIR` environment variable
  - [ ] Handle comma-separated paths
- [ ] Create directory scanner
  - [ ] List all JSONL files
  - [ ] Extract project paths from directory structure
  - [ ] Map files to session UUIDs
- [ ] Add file metadata tracking
  - [ ] File size
  - [ ] Modification time
  - [ ] Last read position
- [ ] Write discovery tests
  - [ ] Multiple directories
  - [ ] Missing directories
  - [ ] Permission issues

### 2.3 Session Manager (`pkg/session`)
- [ ] Choose embedded database (BoltDB recommended)
- [ ] Design session metadata schema
  - [ ] UUID (primary key)
  - [ ] User-defined name
  - [ ] Project path
  - [ ] Created/updated timestamps
  - [ ] Tags
  - [ ] Description
- [ ] Implement SessionManager
  - [ ] `SetName(uuid, name)` - Assign friendly name
  - [ ] `GetByName(name)` - Lookup by name
  - [ ] `GetByUUID(uuid)` - Lookup by UUID
  - [ ] `List()` - List all sessions
  - [ ] `Delete(uuid)` - Remove session
  - [ ] `Update(uuid, metadata)` - Update metadata
- [ ] Add indexing for fast lookups
  - [ ] UUID index
  - [ ] Name index (unique constraint)
  - [ ] Project path index
- [ ] Implement database migrations
- [ ] Add data backup/restore functionality
- [ ] Write session manager tests
  - [ ] CRUD operations
  - [ ] Concurrent access
  - [ ] Data persistence
  - [ ] Index integrity

---

## Phase 3: Real-time Monitoring Engine

### 3.1 File Watcher (`pkg/watcher`)
- [ ] Choose file watching library (`fsnotify`)
- [ ] Implement Watcher interface
  - [ ] `Start(ctx, paths)` - Begin watching
  - [ ] `Stop()` - Graceful shutdown
  - [ ] `Events()` - Event channel
- [ ] Add event types
  - [ ] File created
  - [ ] File modified
  - [ ] File deleted
  - [ ] File moved
- [ ] Implement event debouncing
  - [ ] 100ms debounce window
  - [ ] Batch multiple events for same file
- [ ] Handle edge cases
  - [ ] File rotation
  - [ ] Directory creation/deletion
  - [ ] Symlinks
  - [ ] Network filesystems
- [ ] Add error recovery
  - [ ] Automatic restart on watcher errors
  - [ ] Reconnection logic
  - [ ] Circuit breaker pattern
- [ ] Write watcher tests
  - [ ] Event generation
  - [ ] Debouncing
  - [ ] Error handling
  - [ ] Concurrent file changes

### 3.2 Incremental File Reader (`pkg/reader`)
- [ ] Implement file position tracking
  - [ ] Store last read offset per file
  - [ ] Persist positions to DB
  - [ ] Handle file truncation
- [ ] Create incremental reader
  - [ ] Seek to last position
  - [ ] Read new lines only
  - [ ] Update position after successful read
- [ ] Add file handle management
  - [ ] Connection pooling
  - [ ] Automatic cleanup
  - [ ] Resource limits
- [ ] Implement retry logic
  - [ ] File locked → exponential backoff
  - [ ] File not found → wait for creation
  - [ ] Permission denied → log and skip
- [ ] Write reader tests
  - [ ] Incremental reads
  - [ ] File rotation handling
  - [ ] Concurrent access
  - [ ] Large file handling

### 3.3 Token Aggregator (`pkg/aggregator`)
- [ ] Define aggregation data structures
  - [ ] `SessionStats` - Per-session aggregation
  - [ ] `BillingBlock` - 5-hour billing windows
  - [ ] `TokenBreakdown` - By token type
- [ ] Implement Aggregator
  - [ ] `ProcessEntry(entry)` - Update stats
  - [ ] `GetSessionStats(id)` - Retrieve stats
  - [ ] `GetAllSessions()` - List all
  - [ ] `CalculateBurnRate(id, window)` - Compute rate
- [ ] Add billing block detection
  - [ ] UTC-based 5-hour windows
  - [ ] Detect block boundaries
  - [ ] Track active vs. inactive blocks
- [ ] Implement token calculations
  - [ ] Sum by type (input, output, cache creation, cache read)
  - [ ] Calculate total tokens
  - [ ] Compute costs (future: integrate pricing)
- [ ] Add burn rate calculation
  - [ ] Tokens per minute
  - [ ] Sliding window average
  - [ ] Projection to limit
- [ ] Implement caching
  - [ ] LRU cache for session stats
  - [ ] Cache size limit (100 sessions)
  - [ ] Automatic eviction
- [ ] Write aggregator tests
  - [ ] Entry processing
  - [ ] Billing block detection
  - [ ] Burn rate calculation
  - [ ] Cache behavior

### 3.4 Entry Deduplication (`pkg/dedup`)
- [ ] Implement hash-based deduplication
  - [ ] Hash function for entries (message ID + timestamp)
  - [ ] Store processed hashes in memory
  - [ ] Check before processing
- [ ] Add retention policy
  - [ ] 24-hour retention window
  - [ ] Periodic cleanup (every hour)
  - [ ] Memory limit protection
- [ ] Create deduplication cache
  - [ ] Thread-safe hash set
  - [ ] Efficient lookup (O(1))
  - [ ] Automatic expiration
- [ ] Write deduplication tests
  - [ ] Duplicate detection
  - [ ] Retention cleanup
  - [ ] Concurrent access

---

## Phase 4: CLI Interface

### 4.1 CLI Framework Setup
- [ ] Choose CLI library (`cobra` recommended)
- [ ] Set up command structure
  - [ ] `token-monitor` - Root command
  - [ ] `monitor [session-name]` - Live monitoring
  - [ ] `list` - List all sessions
  - [ ] `session` - Session management subcommands
  - [ ] `config` - Configuration management
- [ ] Implement global flags
  - [ ] `--config` - Custom config file
  - [ ] `--log-level` - Logging level
  - [ ] `--json` - JSON output
  - [ ] `--no-color` - Disable colors
- [ ] Add version command
- [ ] Implement help text and examples
- [ ] Write CLI tests

### 4.2 Monitor Command (`cmd/monitor.go`)
- [ ] Implement live monitoring command
  - [ ] Parse session name or UUID
  - [ ] Lookup session from database
  - [ ] Start file watcher
  - [ ] Display live stats
- [ ] Add command flags
  - [ ] `--session` - Session name/UUID
  - [ ] `--mode` - Display mode (live, compact, table)
  - [ ] `--refresh` - Refresh rate (default: 1s)
  - [ ] `--json` - JSON output mode
- [ ] Implement session auto-detection
  - [ ] If no session specified, show all active sessions
  - [ ] Allow selection from list
- [ ] Add keyboard shortcuts
  - [ ] `q` - Quit
  - [ ] `r` - Reset stats
  - [ ] `↑/↓` - Navigate sessions (multi-session mode)
  - [ ] `?` - Show help
- [ ] Write monitor command tests

### 4.3 Session Management Commands
- [ ] Implement `session list`
  - [ ] Display all sessions with metadata
  - [ ] Table format with columns: UUID, Name, Project, Last Activity, Tokens
  - [ ] Sort options (by name, date, tokens)
  - [ ] Filter options (by project, date range)
- [ ] Implement `session name <uuid> <name>`
  - [ ] Assign friendly name to session
  - [ ] Validate name uniqueness
  - [ ] Update database
- [ ] Implement `session show <name|uuid>`
  - [ ] Display detailed session info
  - [ ] Token breakdown by type
  - [ ] Billing blocks
  - [ ] Activity timeline
- [ ] Implement `session delete <name|uuid>`
  - [ ] Remove session metadata
  - [ ] Confirmation prompt
  - [ ] Preserve JSONL data (read-only)
- [ ] Implement `session export <name|uuid>`
  - [ ] Export session data to CSV/JSON
  - [ ] Include all metrics and metadata
- [ ] Write session command tests

### 4.4 Configuration Commands
- [ ] Implement `config show`
  - [ ] Display current configuration
  - [ ] Show source (default, file, env, flag)
  - [ ] Validate configuration
- [ ] Implement `config set <key> <value>`
  - [ ] Update configuration value
  - [ ] Persist to config file
  - [ ] Validate new value
- [ ] Implement `config reset`
  - [ ] Reset to defaults
  - [ ] Confirmation prompt
- [ ] Write config command tests

---

## Phase 5: Display & UI

### 5.1 TUI Framework (`pkg/display`)
- [ ] Choose TUI library (`bubbletea` recommended)
- [ ] Implement Display interface
  - [ ] `NewDisplay(mode)` - Create display
  - [ ] `Update(stats)` - Update data
  - [ ] `Render()` - Generate output
  - [ ] `Start(ctx)` - Start event loop
- [ ] Add display modes
  - [ ] `ModeLive` - Full-screen dashboard
  - [ ] `ModeCompact` - Single-line status
  - [ ] `ModeTable` - Static table
  - [ ] `ModeJSON` - JSON output
- [ ] Implement color scheme
  - [ ] Token types (input, output, cache)
  - [ ] Status indicators (active, inactive)
  - [ ] Thresholds (low, medium, high usage)
- [ ] Add terminal size detection
  - [ ] Adapt layout to terminal size
  - [ ] Fallback to compact mode if too small
- [ ] Write display tests

### 5.2 Live Dashboard Mode
- [ ] Design dashboard layout
  ```
  ┌─────────────────────────────────────────────────────────┐
  │ Token Monitor - Session: my-project                     │
  ├─────────────────────────────────────────────────────────┤
  │ Session ID: a1b2c3d4-...          Last Update: 14:23:45 │
  │ Project: /path/to/project          Active: 2h 34m       │
  ├─────────────────────────────────────────────────────────┤
  │ TOKEN USAGE                                             │
  │   Input:          125,432  ████████░░  (62%)            │
  │   Output:          45,123  ███░░░░░░░  (22%)            │
  │   Cache Create:    28,901  ██░░░░░░░░  (14%)            │
  │   Cache Read:       3,456  ░░░░░░░░░░   (2%)            │
  │   ─────────────────────────────────────────             │
  │   Total:          202,912                               │
  │                                                          │
  │ BURN RATE: 1,245 tokens/min  [MODERATE]                 │
  │                                                          │
  │ CURRENT BILLING BLOCK (00:00 - 05:00 UTC)               │
  │   Tokens: 89,234 / 500,000  [17%] ████░░░░░░░░░░░       │
  │   Time:   2h 23m / 5h       [47%] ███████░░░░░░░        │
  │   Projected: 156,789 tokens (31% of limit)              │
  │                                                          │
  │ Press 'q' to quit, '?' for help                         │
  └─────────────────────────────────────────────────────────┘
  ```
- [ ] Implement real-time updates
  - [ ] Refresh every 1 second
  - [ ] Animate progress bars
  - [ ] Flash new entries
- [ ] Add burn rate indicators
  - [ ] Color coding (green/yellow/red)
  - [ ] Historical chart (sparkline)
  - [ ] Projection calculation
- [ ] Implement billing block display
  - [ ] Current block progress
  - [ ] Next block countdown
  - [ ] Gap blocks indicator
- [ ] Write dashboard tests

### 5.3 Compact Status Mode
- [ ] Design compact format
  ```
  [my-project] 202.9K tokens (in: 125K, out: 45K) | 1.2K/min | Block: 89K/500K (17%)
  ```
- [ ] Implement single-line output
- [ ] Add color coding
- [ ] Support terminal width adaptation
- [ ] Write compact mode tests

### 5.4 Table Output Mode
- [ ] Choose table library (`tablewriter` or custom)
- [ ] Implement session table
  - [ ] Columns: Name, UUID (short), Project, Tokens, Last Activity
  - [ ] Sortable columns
  - [ ] Pagination for many sessions
- [ ] Implement token breakdown table
  - [ ] Rows: Token types
  - [ ] Columns: Count, Percentage, Cost (future)
- [ ] Add table formatting options
  - [ ] ASCII borders
  - [ ] Markdown format
  - [ ] CSV format
- [ ] Write table tests

### 5.5 JSON Output Mode
- [ ] Design JSON schema
  ```json
  {
    "session": {
      "id": "uuid",
      "name": "my-project",
      "projectPath": "/path",
      "startTime": "ISO8601",
      "lastActivity": "ISO8601"
    },
    "tokens": {
      "input": 125432,
      "output": 45123,
      "cacheCreation": 28901,
      "cacheRead": 3456,
      "total": 202912
    },
    "burnRate": 1245.3,
    "currentBlock": {
      "startTime": "ISO8601",
      "endTime": "ISO8601",
      "tokensUsed": 89234,
      "tokenLimit": 500000,
      "percentUsed": 17.8,
      "projected": 156789
    }
  }
  ```
- [ ] Implement JSON serialization
- [ ] Add pretty-print option
- [ ] Support streaming JSON (JSONL for multiple sessions)
- [ ] Write JSON output tests

---

## Phase 6: Performance & Optimization

### 6.1 Concurrent Processing
- [ ] Implement worker pool
  - [ ] Configurable pool size (default: 5)
  - [ ] Job queue with backpressure
  - [ ] Graceful shutdown
- [ ] Add concurrent file processing
  - [ ] Process multiple files in parallel
  - [ ] Coordinate results collection
  - [ ] Handle errors gracefully
- [ ] Implement channel-based communication
  - [ ] Event channels
  - [ ] Result channels
  - [ ] Error channels
- [ ] Write concurrency tests
  - [ ] Race condition detection
  - [ ] Deadlock prevention
  - [ ] Resource cleanup

### 6.2 Memory Optimization
- [ ] Implement LRU cache for session stats
  - [ ] Max 100 sessions in memory
  - [ ] Automatic eviction
  - [ ] Cache hit/miss metrics
- [ ] Add memory pooling
  - [ ] Buffer pools for file reading
  - [ ] Object pools for data structures
- [ ] Implement streaming for large files
  - [ ] Process line-by-line
  - [ ] Avoid loading entire file
  - [ ] Backpressure handling
- [ ] Add memory profiling
  - [ ] pprof integration
  - [ ] Memory leak detection
  - [ ] Allocation tracking
- [ ] Write memory tests
  - [ ] Memory usage benchmarks
  - [ ] Leak detection tests

### 6.3 I/O Optimization
- [ ] Implement file position caching
  - [ ] Store last read positions
  - [ ] Skip unchanged portions
  - [ ] Handle file rotation
- [ ] Add batched database writes
  - [ ] 100ms write window
  - [ ] Batch multiple updates
  - [ ] Transaction support
- [ ] Optimize file handle management
  - [ ] Connection pooling
  - [ ] Automatic cleanup
  - [ ] Resource limits
- [ ] Write I/O benchmarks
  - [ ] File reading throughput
  - [ ] Database write performance
  - [ ] Concurrent access

### 6.4 Performance Monitoring
- [ ] Add metrics collection
  - [ ] Processing latency
  - [ ] Throughput (entries/sec)
  - [ ] Cache hit rate
  - [ ] Error rate
- [ ] Implement performance logging
  - [ ] Slow operation detection
  - [ ] Resource usage tracking
  - [ ] Performance regression alerts
- [ ] Add profiling support
  - [ ] CPU profiling
  - [ ] Memory profiling
  - [ ] Goroutine profiling
- [ ] Write performance tests
  - [ ] Benchmark suites
  - [ ] Load testing
  - [ ] Stress testing

---

## Phase 7: Testing & Quality Assurance

### 7.1 Unit Tests
- [ ] Achieve >80% code coverage
- [ ] Test all public APIs
- [ ] Add table-driven tests
- [ ] Mock external dependencies
- [ ] Write test utilities
- [ ] Add test documentation

### 7.2 Integration Tests
- [ ] End-to-end monitoring flow
  - [ ] Write JSONL → detect → parse → aggregate → display
- [ ] Multi-session scenarios
- [ ] Database persistence
- [ ] Configuration loading
- [ ] Error recovery flows
- [ ] Write integration test suite

### 7.3 Performance Tests
- [ ] Benchmark file parsing
  - [ ] Target: >10K lines/second
- [ ] Stress test with 100 sessions
- [ ] Memory leak detection (24-hour run)
- [ ] I/O throughput measurement
- [ ] Concurrent access benchmarks
- [ ] Write performance test suite

### 7.4 Manual Testing
- [ ] Test on macOS
- [ ] Test on Linux
- [ ] Test on Windows
- [ ] Test with real Claude Code sessions
- [ ] Test with large JSONL files (>10MB)
- [ ] Test with many sessions (>100)
- [ ] Document manual test procedures

---

## Phase 8: Documentation & Release

### 8.1 User Documentation
- [ ] Update README.md
  - [ ] Project overview
  - [ ] Installation instructions
  - [ ] Quick start guide
  - [ ] Usage examples
  - [ ] Configuration reference
- [ ] Create USAGE.md
  - [ ] Command reference
  - [ ] Common workflows
  - [ ] Troubleshooting
  - [ ] FAQ
- [ ] Write CONTRIBUTING.md
  - [ ] Development setup
  - [ ] Code style guide
  - [ ] PR process
  - [ ] Testing requirements
- [ ] Create CHANGELOG.md
  - [ ] Version history
  - [ ] Breaking changes
  - [ ] New features
  - [ ] Bug fixes

### 8.2 Developer Documentation
- [ ] Update docs/ARCHITECTURE.md
  - [ ] Finalize architecture decisions
  - [ ] Add sequence diagrams
  - [ ] Document data flows
- [ ] Create docs/API.md
  - [ ] Public package APIs
  - [ ] Data structures
  - [ ] Interface contracts
- [ ] Write docs/TESTING.md
  - [ ] Testing strategy
  - [ ] Running tests
  - [ ] Writing new tests
- [ ] Add code comments
  - [ ] Package documentation
  - [ ] Function documentation
  - [ ] Complex logic explanation

### 8.3 Build & Release
- [ ] Set up goreleaser
  - [ ] Multi-platform builds (macOS, Linux, Windows)
  - [ ] Architecture support (amd64, arm64)
  - [ ] Archive creation (tar.gz, zip)
- [ ] Create GitHub Actions workflow
  - [ ] Build on push
  - [ ] Run tests
  - [ ] Release on tag
- [ ] Create installation methods
  - [ ] Homebrew tap (macOS/Linux)
  - [ ] Scoop bucket (Windows)
  - [ ] Direct binary download
  - [ ] `go install` instructions
- [ ] Prepare v0.1.0 release
  - [ ] Tag release
  - [ ] Generate release notes
  - [ ] Publish binaries

---

## Phase 9: Future Enhancements

### 9.1 Cost Analysis (Future)
- [ ] Integrate LiteLLM pricing API
- [ ] Calculate actual costs
- [ ] Add budget tracking
- [ ] Implement cost alerts
- [ ] Generate cost reports
- [ ] Add cost projections

### 9.2 Web Dashboard (Future)
- [ ] Implement HTTP REST API
- [ ] Add WebSocket support for live updates
- [ ] Create web frontend
  - [ ] Real-time charts
  - [ ] Session management UI
  - [ ] Cost analytics
- [ ] Add authentication
- [ ] Implement multi-user support

### 9.3 Alerting (Future)
- [ ] Token limit warnings
- [ ] Cost threshold notifications
- [ ] Slack integration
- [ ] Discord integration
- [ ] Email notifications
- [ ] Webhook support

### 9.4 Advanced Analytics (Future)
- [ ] Historical trend analysis
- [ ] Usage patterns detection
- [ ] Anomaly detection
- [ ] Forecasting models
- [ ] Custom metrics
- [ ] Export to analytics platforms

### 9.5 Export & Reporting (Future)
- [ ] CSV export
- [ ] PDF reports
- [ ] Prometheus metrics export
- [ ] Grafana dashboard templates
- [ ] Custom report templates

---

## Current Status

**Last Updated**: 2025-11-17

**Current Phase**: Phase 1 - Foundation & Core Infrastructure

**Completed Tasks**:
- Project directory structure created
- .gitignore configured
- docs/ARCHITECTURE.md created
- docs/todolist.md created (this file)

**Next Steps**:
1. Wait for user to create .claude folder with development rules
2. Initialize Go module
3. Set up basic project structure
4. Implement configuration system
5. Begin parser development

**Blockers**: None

**Notes**:
- Analyzed ccusage and claude-code projects for design patterns
- Token tracking approach based on JSONL file monitoring
- Session management via UUID → name mapping
- Real-time monitoring using fsnotify
