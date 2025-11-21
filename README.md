# Token Monitor

Real-time token usage monitoring CLI for Claude Code sessions.

## Overview

**Token Monitor** tracks Claude Code's token consumption in real-time, providing session-based monitoring, burn rate analysis, and billing block tracking.

### Key Features

- **Real-time Monitoring**: Live updates as Claude Code generates tokens
- **Session Management**: Track sessions by user-friendly names instead of UUIDs
- **Token Breakdown**: Separate tracking for input, output, cache creation, and cache read tokens
- **Billing Blocks**: Automatic 5-hour UTC billing window detection
- **Burn Rate Analysis**: Tokens per minute with hourly projections
- **Multiple Output Formats**: Table, JSON, and simple text output
- **Delta Tracking**: View cumulative and real-time token changes

## Installation

### From Source

```bash
git clone https://github.com/0xmhha/token-monitor.git
cd token-monitor
go build -o token-monitor ./cmd/token-monitor
```

### Using Go Install

```bash
go install github.com/0xmhha/token-monitor/cmd/token-monitor@latest
```

## Quick Start

```bash
# Monitor all sessions in real-time
token-monitor watch

# View statistics
token-monitor stats

# List sessions
token-monitor session list

# Assign a friendly name to a session
token-monitor session name <uuid> my-project
```

## Commands

### `stats` - Display Statistics

```bash
# Overall statistics
token-monitor stats

# Filter by session
token-monitor stats --session <uuid>

# Group by model
token-monitor stats --group model

# Top 10 sessions by usage
token-monitor stats --top 10

# JSON output
token-monitor stats --format json
```

### `watch` - Live Monitoring

```bash
# Watch all sessions
token-monitor watch

# Watch specific session
token-monitor watch --session <uuid>

# Custom refresh rate
token-monitor watch --refresh 2s

# Simple text format
token-monitor watch --format simple
```

The watch command displays:
- Token usage with cumulative and real-time deltas
- Statistics (average, min, max, percentiles)
- Burn rate (tokens/minute and projected hourly)
- Current billing block with time remaining

### `session` - Session Management

```bash
# List all sessions
token-monitor session list

# Sort by name, date, or uuid
token-monitor session list --sort name

# Name a session
token-monitor session name <uuid> my-project

# Show session details
token-monitor session show my-project

# Delete session metadata (keeps data files)
token-monitor session delete my-project
```

### `list` - List Discovered Sessions

```bash
token-monitor list
```

## Configuration

Token Monitor searches for configuration in:
1. `./token-monitor.yaml`
2. `~/.config/token-monitor/config.yaml`
3. `/etc/token-monitor/config.yaml`

### Example Configuration

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
```

### Environment Variables

- `CLAUDE_CONFIG_DIR`: Override Claude config directories (comma-separated)

## Output Example

```
📊 Live Token Monitor - 2024-01-15 14:23:45

┌─────────────────┬──────────────┬──────────────┬────────────┐
│ Metric          │ Total        │ Session +    │ Now +      │
├─────────────────┼──────────────┼──────────────┼────────────┤
│ Requests        │          142 │         +142 │        +12 │
│ Input Tokens    │       125432 │      +125432 │      +8234 │
│ Output Tokens   │        45123 │       +45123 │      +3421 │
│ Total Tokens    │       170555 │      +170555 │     +11655 │
└─────────────────┴──────────────┴──────────────┴────────────┘

🔥 Burn Rate (5-minute window)
┌─────────────────┬──────────────┐
│ Metric          │ Value        │
├─────────────────┼──────────────┤
│ Tokens/min      │       1245.3 │
│ Tokens/hour     │      74718.0 │
│ Entries         │           12 │
└─────────────────┴──────────────┘

📊 Current Billing Block (10:00 - 15:00 UTC)
┌─────────────────┬──────────────┐
│ Metric          │ Value        │
├─────────────────┼──────────────┤
│ Total Tokens    │        89234 │
│ Entries         │           87 │
│ Time Left       │        0h37m │
└─────────────────┴──────────────┘
```

## Billing Blocks

Claude Code uses 5-hour billing windows aligned to UTC:
- 00:00 - 05:00 UTC
- 05:00 - 10:00 UTC
- 10:00 - 15:00 UTC
- 15:00 - 20:00 UTC
- 20:00 - 00:00 UTC

Token Monitor tracks usage within these blocks and shows time remaining.

## How It Works

1. **Data Source**: Reads `~/.config/claude/projects/{projectDir}/{sessionId}.jsonl`
2. **File Watching**: Detects new entries using filesystem events (fsnotify)
3. **Incremental Reading**: Only processes new log entries
4. **Aggregation**: Computes statistics, burn rates, and billing blocks
5. **Display**: Renders live dashboard with real-time updates

### Data Privacy

- **No conversation content stored** - only token counts and metadata
- **Read-only access** - never modifies Claude Code data
- **Local storage only** - all data stays on your machine

## Architecture

```
token-monitor/
├── cmd/token-monitor/    # CLI entry point and commands
├── pkg/
│   ├── aggregator/       # Token statistics and burn rate calculation
│   ├── config/           # Configuration loading
│   ├── discovery/        # Session file discovery
│   ├── display/          # Output formatting
│   ├── logger/           # Structured logging
│   ├── monitor/          # Live monitoring engine
│   ├── parser/           # JSONL log parsing
│   ├── reader/           # Incremental file reading
│   ├── session/          # Session metadata (BoltDB)
│   └── watcher/          # File system watching
└── docs/                 # Documentation
```

## Development

### Prerequisites

- Go 1.22 or later

### Building

```bash
go build -o token-monitor ./cmd/token-monitor
```

### Testing

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...
```

## Documentation

- [Usage Guide](USAGE.md) - Complete command reference, workflows, and troubleshooting
- [Contributing](CONTRIBUTING.md) - Development guide and contribution process
- [Changelog](CHANGELOG.md) - Version history and release notes
- [Architecture](docs/ARCHITECTURE.md) - System design and components
- [TODO List](docs/todolist.md) - Feature roadmap and development tasks

## Technology Stack

- **Language**: Go 1.22+
- **File Watching**: fsnotify
- **Database**: BoltDB (embedded)
- **Testing**: Go testing + testify

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for:

- Development setup and prerequisites
- Code style and quality guidelines
- Testing requirements and best practices
- Pull request process and review workflow

## License

See [LICENSE](LICENSE) file.

## Acknowledgments

- [ccusage](https://github.com/tianhuil/ccusage) - Inspiration for token tracking
- Claude Code CLI - The tool being monitored
- Anthropic - For Claude and Claude Code
