# Token Monitor

[![GitHub release](https://img.shields.io/github/v/release/0xmhha/token-monitor)](https://github.com/0xmhha/token-monitor/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/0xmhha/token-monitor)](https://github.com/0xmhha/token-monitor)
[![License](https://img.shields.io/github/license/0xmhha/token-monitor)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/0xmhha/token-monitor)](https://goreportcard.com/report/github.com/0xmhha/token-monitor)

Real-time token usage monitoring for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) sessions.

## Why Token Monitor?

Claude Code consumes tokens with every interaction, but there's no built-in way to track usage in real-time. Token Monitor fills this gap:

- **Visibility** — See exactly how many tokens each session uses, broken down by input, output, and cache tokens
- **Cost Awareness** — Track burn rate (tokens/min) and billing block remaining time to avoid surprises
- **Integration** — Works as a Claude Code extension via hooks, MCP server, and status line output
- **Privacy** — Reads only token counts from local JSONL logs. No conversation content is accessed or stored.

## Features

| Feature | Description |
|---------|-------------|
| **Interactive TUI** | Full-screen dashboard with live updates (default command) |
| **Live Monitoring** | Real-time token tracking with cumulative and per-update deltas |
| **Session Management** | Name, filter, export, and compare sessions |
| **Burn Rate Analysis** | Tokens/minute with hourly projections over sliding 5-min window |
| **Billing Block Tracking** | 5-hour UTC billing window detection with time remaining |
| **Claude Code Integration** | PostToolUse hooks, MCP server, compact status line output |
| **Fast Query** | Single-metric lookup in <100ms for hook/script use |
| **MCP Server** | JSON-RPC 2.0 server exposing 9 tools for Claude Code |
| **Cross-Session Breakdown** | Aggregate tokens across all sessions, grouped by model (`status --breakdown`, MCP `get_today_usage`) |
| **Install Automation** | One command to wire up statusline + MCP + hook on any machine (`token-monitor install all`) |
| **Multiple Formats** | Table, JSON, simple text, compact, and hook output |

## Installation

### Pre-built Binaries

Download from the [releases page](https://github.com/0xmhha/token-monitor/releases/latest):

```bash
# macOS (Apple Silicon)
curl -L https://github.com/0xmhha/token-monitor/releases/latest/download/token-monitor_darwin_arm64.tar.gz | tar xz
sudo mv token-monitor /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/0xmhha/token-monitor/releases/latest/download/token-monitor_darwin_amd64.tar.gz | tar xz
sudo mv token-monitor /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/0xmhha/token-monitor/releases/latest/download/token-monitor_linux_amd64.tar.gz | tar xz
sudo mv token-monitor /usr/local/bin/
```

### From Source

```bash
git clone https://github.com/0xmhha/token-monitor.git
cd token-monitor
go build -o token-monitor ./cmd/token-monitor
```

### Verify

```bash
token-monitor --version
```

## Quick Start

```bash
# Launch interactive TUI dashboard (default)
token-monitor

# View token statistics
token-monitor stats

# Live monitoring with table output
token-monitor watch

# Fast single-value query (for scripts/hooks)
token-monitor query --current --metric total

# Compact status line
token-monitor status --current
```

## Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `tui` | Interactive TUI dashboard (default when no command given) |
| `stats` | Display token usage statistics with grouping and filtering |
| `watch` | Live monitoring with table/simple output |
| `list` | List all discovered session files |
| `session` | Session management (name, list, show, delete, export) |
| `config` | Configuration management (show, set, validate, reset) |

### Integration Commands

| Command | Description |
|---------|-------------|
| `query` | Fast single-metric lookup (<100ms, no BoltDB) |
| `status` | Compact formatted output for status line display |
| `serve` | MCP JSON-RPC 2.0 server over stdio |

### Query Command

Designed for hooks and scripts. Bypasses BoltDB for fast execution.

```bash
# Single metric
token-monitor query --current --metric total        # 145823
token-monitor query --current --metric burn-rate    # 2145.3
token-monitor query --current --metric block-remaining  # 3h42m

# All metrics as JSON
token-monitor query --current --json

# Hook-compatible format
token-monitor query --current --format hook
```

**Supported metrics**: `total`, `input`, `output`, `count`, `burn-rate`, `burn-rate-hour`, `block-remaining`, `block-tokens`

### Status Command

Compact output for Claude Code status line.

```bash
token-monitor status --current              # default: fire 12.5K tokens | 2.1K/min | Block: 3h42m
token-monitor status --current --compact    # 12.5K/2.1K^  (~13 chars)
token-monitor status --current --full       # Total: 12,534 | Rate: 2,145/min | In: 8,234 Out: 4,300 | Block: 3h42m left
token-monitor status --current --no-emoji   # 12.5K tokens | 2.1K/min | Block: 3h42m
token-monitor status --current --watch      # continuous output
```

### MCP Server

Exposes token data as tools for Claude Code via JSON-RPC 2.0 over stdio.

```bash
token-monitor serve --stdio
```

**Available tools** (9):
- Per-session: `get_token_usage`, `get_burn_rate`, `get_billing_block`, `get_session_detail`
- Cross-session breakdown (v0.2): `get_session_breakdown`, `get_today_usage`, `get_usage_by_window`
- Listing/comparison: `list_sessions`, `compare_sessions`

## Claude Code Integration

### Quickest path (v0.2): `install` subcommand

After installing the binary, register everything with one command:

```bash
# Install statusline + MCP server + PostToolUse hook (idempotent)
token-monitor install all

# Or install components individually
token-monitor install statusline       # patches ~/.claude/statusline-command.sh
token-monitor install mcp --global     # registers in ~/.claude.json
token-monitor install mcp --project    # registers in ./.mcp.json (CWD)
token-monitor install hook             # registers PostToolUse hook

# Preview without writing
token-monitor install all --dry-run

# Print just the statusline snippet (for manual integration)
token-monitor install statusline --print

# Reverse everything
token-monitor install --uninstall-all
```

All install operations are **atomic** (rename pattern), refuse to overwrite user-authored entries, and create `*.bak.YYYYMMDD-HHMMSS` backups before writing.

### Manual integration

Choose one (or both) depending on your needs:

| Method | Best For | How It Works |
|--------|----------|-------------|
| **Hooks** | Passive monitoring — see token usage after every action | Runs `token-monitor` on each tool call, displays result |
| **MCP Server** | On-demand queries — ask Claude about token usage | Claude calls tools like `get_token_usage` directly |

### Option A: Hooks (Passive Monitoring)

Add to `~/.claude/settings.json`. Token usage appears automatically after every tool call.

```json
{
  "hooks": {
    "PostToolUse": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "token-monitor status --current --compact 2>/dev/null || true"
      }]
    }],
    "Stop": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "token-monitor query --current --json 2>/dev/null || true"
      }]
    }]
  }
}
```

### Option B: MCP Server (On-Demand Queries)

Add to `~/.claude.json` (global) or `.mcp.json` (project-level). Claude can then answer questions like "How many tokens have I used?" directly.

```json
{
  "mcpServers": {
    "token-monitor": {
      "command": "token-monitor",
      "args": ["serve", "--stdio"]
    }
  }
}
```

## How It Works

```
Claude Code JSONL logs
  ~/.claude/projects/{project}/{session}.jsonl
       |
       v
  +-----------+     +-----------+     +------------+
  | Discovery | --> |  Reader   | --> | Aggregator |
  | (scan)    |     | (parse)   |     | (stats)    |
  +-----------+     +-----------+     +------------+
       |                                     |
       v                                     v
  +-----------+                       +------------+
  |  Watcher  |                       |  Display   |
  | (fsnotify)|                       | (format)   |
  +-----------+                       +------------+
```

1. **Discovery** scans Claude config directories for session JSONL files
2. **Reader** incrementally parses new log entries (position tracking via BoltDB or memory)
3. **Aggregator** computes statistics, burn rates, and billing blocks
4. **Watcher** detects file changes via fsnotify for live updates
5. **Display** renders output in the requested format

### Data Privacy

- **Read-only** — never modifies Claude Code data files
- **No content access** — only reads token count metadata, not conversation content
- **Local only** — all data stays on your machine

## Billing Blocks

Claude Code uses 5-hour billing windows aligned to UTC:

| Block | UTC Time |
|-------|----------|
| 1 | 00:00 - 05:00 |
| 2 | 05:00 - 10:00 |
| 3 | 10:00 - 15:00 |
| 4 | 15:00 - 20:00 |
| 5 | 20:00 - 00:00 |

Token Monitor tracks usage within these blocks and shows remaining time.

## Configuration

Configuration file locations (in order of precedence):

1. `./token-monitor.yaml`
2. `~/.config/token-monitor/config.yaml`
3. `/etc/token-monitor/config.yaml`

```yaml
claude_config_dirs:
  - ~/.config/claude/projects
  - ~/.claude/projects

storage:
  db_path: ~/.config/token-monitor/sessions.db

logging:
  level: info
  format: text
  output: stderr

integration:
  auto_detect: true
  status:
    format: default    # compact | default | full
    emoji: true
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `CLAUDE_CONFIG_DIR` | Override Claude config directories (comma-separated) |
| `CLAUDE_SESSION_ID` | Override session auto-detection with specific ID |
| `CLAUDE_PROJECT_DIR` | Limit auto-detection to a specific project directory |

## Project Structure

```
token-monitor/
├── cmd/token-monitor/    # CLI commands (main, query, status, serve, etc.)
├── pkg/
│   ├── aggregator/       # Statistics, burn rate, billing block calculation
│   ├── analysis/         # Cost analysis
│   ├── config/           # YAML configuration with validation
│   ├── discovery/        # Session file discovery + auto-detection
│   ├── display/          # Output formatting (table, JSON, compact K/M)
│   ├── logger/           # Structured logging
│   ├── mcp/              # MCP JSON-RPC 2.0 server and tool handlers
│   ├── monitor/          # Live monitoring engine
│   ├── parser/           # JSONL log parsing with validation
│   ├── reader/           # Incremental file reading with position tracking
│   ├── session/          # Session metadata storage (BoltDB)
│   ├── tui/              # Interactive Bubbletea dashboard
│   └── watcher/          # Filesystem watching (fsnotify)
└── docs/                 # Architecture, integration guide, roadmap
```

## Development

### Prerequisites

- Go 1.24+

### Build & Test

```bash
go build -o token-monitor ./cmd/token-monitor
go test ./...
go test -race ./...
go test -cover ./...
```

## Troubleshooting

| Problem | Solution |
|---------|----------|
| No sessions found | Verify Claude Code has been used: `ls ~/.claude/projects` |
| Watch not updating | Ensure Claude Code is actively running; try `--refresh 5s` |
| BoltDB timeout | Another process (e.g., MCP serve) holds the lock — watch/stats auto-fallback to in-memory mode |
| Permission denied | Check file ownership: `ls -la ~/.claude/projects/` |

## Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture.md) | System design, component interactions, data flow |
| [Testing Guide](docs/TESTING.md) | Test strategy, coverage targets, CI/CD |
| [Roadmap](docs/roadmap.md) | Planned features and version milestones |
| [Contributing](CONTRIBUTING.md) | Development setup and PR process |
| [Changelog](CHANGELOG.md) | Version history and release notes |

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, code style, and PR process.

## License

See [LICENSE](LICENSE) file.

## Acknowledgments

- [ccusage](https://github.com/tianhuil/ccusage) — Inspiration for token tracking
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) — The tool being monitored
- [Anthropic](https://www.anthropic.com/) — For Claude and Claude Code
