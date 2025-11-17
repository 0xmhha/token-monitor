# Token Monitor

Real-time token usage monitoring for Claude Code CLI sessions.

## Overview

**Token Monitor** tracks Claude Code's token consumption in real-time, providing session-based monitoring, historical analysis, and burn rate tracking.

### Key Features

- **Real-time Monitoring**: Live updates as Claude Code generates tokens
- **Session Management**: Track sessions by user-friendly names instead of UUIDs
- **Token Breakdown**: Separate tracking for input, output, cache creation, and cache read tokens
- **Billing Blocks**: Automatic 5-hour UTC billing window detection
- **Burn Rate Analysis**: Tokens per minute with projections
- **Beautiful TUI**: Terminal UI with progress bars and color coding
- **Cross-Platform**: macOS, Linux, Windows support

## Status

**Current Phase**: Foundation & Analysis

This project is in early development. The following has been completed:

- [x] Project analysis (ccusage and claude-code)
- [x] Architecture design
- [x] Feature specification
- [x] Development roadmap

### Next Steps

1. Wait for `.claude` folder creation with development rules
2. Initialize Go module
3. Begin core implementation

## Quick Start

> **Note**: Implementation not yet complete. This is the planned usage.

```bash
# Install
go install github.com/yourusername/token-monitor@latest

# Monitor a session
token-monitor monitor my-project

# List all sessions
token-monitor session list

# Assign name to session
token-monitor session name a1b2c3d4-... my-project
```

## Documentation

- **[Architecture](docs/ARCHITECTURE.md)** - System design and components
- **[Design](docs/DESIGN.md)** - Detailed design decisions
- **[TODO List](docs/todolist.md)** - Feature roadmap and development tasks

## How It Works

Token Monitor reads Claude Code's JSONL logs in real-time:

1. **Data Source**: `~/.config/claude/projects/{projectDir}/{sessionId}.jsonl`
2. **File Watching**: Detects new entries using filesystem events
3. **Parsing**: Extracts token usage metrics (input, output, cache tokens)
4. **Aggregation**: Computes session statistics and billing blocks
5. **Display**: Renders live dashboard with burn rate and projections

### Data Privacy

- **No conversation content stored** - only token counts and metadata
- **Read-only access** - never modifies Claude Code data
- **Local storage only** - all data stays on your machine

## Architecture

```
Claude Code → JSONL Files → Token Monitor → Terminal UI
                              │
                              ├─ File Watcher (fsnotify)
                              ├─ JSONL Parser
                              ├─ Token Aggregator
                              ├─ Session Manager (BoltDB)
                              └─ Display Engine (Bubbletea)
```

## Planned Display Modes

### Live Dashboard
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

### Compact Mode
```
[my-project] 202.9K tokens (in: 125K, out: 45K) | 1.2K/min | Block: 89K/500K (17%)
```

### JSON Mode
```json
{
  "session": {
    "id": "a1b2c3d4-...",
    "name": "my-project",
    "projectPath": "/path/to/project"
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
    "tokensUsed": 89234,
    "tokenLimit": 500000,
    "percentUsed": 17.8
  }
}
```

## Technology Stack

- **Language**: Go 1.21+
- **File Watching**: fsnotify
- **Database**: BoltDB (embedded)
- **TUI Framework**: Bubbletea
- **CLI Framework**: Cobra
- **Testing**: Go standard testing + testify

## Performance Targets

- **Latency**: <100ms from JSONL write to UI update
- **Memory**: <50MB baseline, <200MB with 100 active sessions
- **CPU**: <5% average
- **Parsing**: >10,000 entries/second

## Development

### Prerequisites

- Go 1.21 or higher
- Make (optional)

### Build

```bash
# Clone repository
git clone https://github.com/yourusername/token-monitor.git
cd token-monitor

# Initialize module (not yet done)
go mod init github.com/yourusername/token-monitor
go mod tidy

# Build
go build -o bin/token-monitor ./cmd/token-monitor

# Run tests
go test ./...

# Run with race detector
go test -race ./...
```

## Contributing

This project is in early development. Contributions welcome after initial implementation.

## License

See [LICENSE](LICENSE) file.

## Acknowledgments

- **[ccusage](https://github.com/tianhuil/ccusage)** - Inspiration for token tracking patterns
- **Claude Code CLI** - The tool being monitored
- **Anthropic** - For Claude and Claude Code

## Contact

Create an issue for questions or suggestions.

---

**Status**: 🚧 Under Development | **Phase**: Foundation & Analysis
