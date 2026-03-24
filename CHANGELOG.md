# Changelog

All notable changes to Token Monitor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.1.1] - 2026-03-24

### Added

#### Claude Code Integration
- **`query` command**: Fast single-metric token lookup (<100ms, no BoltDB)
  - Metrics: total, input, output, count, burn-rate, burn-rate-hour, block-remaining, block-tokens
  - JSON output and hook-compatible format
- **`status` command**: Compact formatted output for Claude Code status line
  - Three formats: compact (~13 chars), default (~45 chars), full (~75 chars)
  - Watch mode with configurable interval
  - `--no-emoji` flag
- **`serve` command**: MCP JSON-RPC 2.0 server over stdio
  - Tools: get_token_usage, get_burn_rate, get_billing_block, list_sessions, get_session_detail, compare_sessions
- **Session auto-detection** (`--current` flag)
  - Priority: CLAUDE_SESSION_ID env var > CLAUDE_PROJECT_DIR > most recent session
  - 1-second result cache for repeated calls
- **K/M number formatting** for compact display (e.g., 12.5K, 1.2M)
- **Integration config schema** (auto_detect, daemon, mcp, status settings)
- **MCP server project config** (`.mcp.json`)

#### Interactive TUI
- **Bubbletea dashboard** as default command with real-time updates
- Session list, stats panel, help overlay

#### Session Enhancements
- Session list filters (`--project`, `--from`, `--to`, `--min-tokens`)
- Enhanced session show output (token breakdown, timeline, statistics)
- Session export to CSV, JSON, and agent-forge format
- Keyboard shortcuts in watch mode (`q`, `r`, `?`, ESC)
- `config set` and `config validate` commands

### Fixed
- BoltDB lock conflict when MCP serve is running — watch/stats now fallback to in-memory position store
- OPOST terminal flag restored after MakeRaw to prevent staircase output
- Help overlay closes on any key press

### Changed
- README rewritten with feature table, integration guide, and troubleshooting
- Documentation consolidated: removed redundant API.md, DESIGN.md, todolist.md, USAGE.md, RELEASE_NOTES.md
- Roadmap updated to reflect completed features

---

## [0.1.0] - 2025-11-21

### Added

#### Core Features
- **Live Token Monitoring**: Real-time monitoring of Claude Code token usage
  - Auto-discovery of Claude Code session files
  - Real-time file watching with fsnotify
  - Incremental reading with position tracking

- **Token Aggregation**: Comprehensive token usage tracking
  - Per-session statistics with breakdown by token type (input, output, cache creation, cache read)
  - Burn rate calculation (tokens/min, tokens/hour) with 5-minute sliding window

- **Billing Block Tracking**: UTC-based 5-hour billing window monitoring
  - Automatic billing block detection with progress tracking

- **Session Management**: BoltDB-based persistent storage
  - Friendly session naming with UUID mapping
  - Fast lookups by name or UUID, CRUD operations

#### CLI Commands
- `stats` — Aggregated statistics with grouping (model, session, date, hour) and filtering
- `list` — List all discovered sessions
- `watch` — Live monitoring with delta tracking, burn rate, billing block status
- `session name/list/show/delete` — Session metadata management
- `config show/path/reset` — Configuration management

#### Infrastructure
- YAML configuration with env var and CLI flag overrides
- Structured logging with slog (debug, info, warn, error)
- GitHub Actions CI/CD with goreleaser
- golangci-lint with 15+ linters

#### Testing
- 78%+ coverage for monitor, 87% for aggregator, 86% for config
- Race detector enabled, table-driven tests, mock implementations

### Known Limitations
- No interactive session selection from list
- Cost calculation not yet implemented
- Log rotation not implemented

### Dependencies
- `github.com/fsnotify/fsnotify` v1.7.0
- `go.etcd.io/bbolt` v1.3.10
- `gopkg.in/yaml.v3` v3.0.1
- `github.com/stretchr/testify` v1.8.1
- `github.com/charmbracelet/bubbletea` v1.3.10
- `github.com/charmbracelet/lipgloss` v1.1.0

---

[0.1.1]: https://github.com/0xmhha/token-monitor/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/0xmhha/token-monitor/releases/tag/v0.1.0
