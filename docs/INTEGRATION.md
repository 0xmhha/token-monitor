# Claude Code Integration Guide

> Design document for integrating token-monitor with Claude Code's extension ecosystem.

## Overview

Token-monitor currently operates as a standalone CLI/TUI tool. This document describes the features needed to integrate it into Claude Code as a first-class extension, enabling real-time token awareness across hooks, MCP tools, skills, and the status line.

## Integration Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                         Claude Code                               │
│                                                                   │
│  ┌───────────┐  ┌────────────┐  ┌──────────┐  ┌──────────────┐  │
│  │   Hooks   │  │ MCP Server │  │  Skills  │  │ Status Line  │  │
│  │ PostTool  │  │  (stdio)   │  │ /tokens  │  │ 🔥 12K 2/min │  │
│  └─────┬─────┘  └──────┬─────┘  └────┬─────┘  └──────┬───────┘  │
│        │               │             │               │           │
└────────┼───────────────┼─────────────┼───────────────┼───────────┘
    ①────┘          ②────┘        ③────┘          ④────┘
         │               │             │               │
┌────────┼───────────────┼─────────────┼───────────────┼───────────┐
│        ▼               ▼             ▼               ▼           │
│   ┌─────────┐   ┌───────────┐  ┌─────────┐   ┌───────────┐     │
│   │  query  │   │   serve   │  │  query  │   │  status   │     │
│   │ (fast)  │   │   (MCP)   │  │ (JSON)  │   │ (oneline) │     │
│   └────┬────┘   └─────┬─────┘  └────┬────┘   └─────┬─────┘     │
│        │              │              │              │            │
│        └──────────────┴──────────────┴──────────────┘            │
│                           │                                      │
│                   ┌───────▼────────┐                             │
│                   │  Core Packages │  (existing)                 │
│                   │  - monitor     │                             │
│                   │  - aggregator  │                             │
│                   │  - reader      │                             │
│                   │  - discovery   │                             │
│                   └────────────────┘                             │
│                     token-monitor                                │
└──────────────────────────────────────────────────────────────────┘
```

## Claude Code Extension Points

| Extension Point | Mechanism | Data Flow | Use Case |
|----------------|-----------|-----------|----------|
| **Hooks** | Shell command on events (PreToolUse, PostToolUse, Stop) | stdin JSON → stdout text | Per-command token tracking |
| **MCP Server** | Persistent process exposing tools via stdio | JSON-RPC over stdio | Claude querying token data directly |
| **Skills** | Slash commands (`/tokens`) | CLI invocation | User-initiated token queries |
| **Status Line** | Inline display in Claude Code UI | Formatted single-line string | Always-visible token counter |

## Required Features

### Feature 1: `token-monitor query` — Fast Single-Value Lookup

**Purpose**: Called from PostToolUse hooks on every tool invocation. Must be fast (<100ms).

**Interface**:

```bash
# Auto-detect current session + get total tokens
token-monitor query --current --metric total
# → 145823

# Get burn rate
token-monitor query --current --metric burn-rate
# → 2145.3

# Full JSON output
token-monitor query --current --json
# → {"session_id":"abc...","total_tokens":145823,"input_tokens":98234,
#    "output_tokens":47589,"burn_rate":2145.3,"billing_block_remaining":"3h42m"}

# Specific session
token-monitor query --session abc123 --metric total
```

**Supported metrics**:

| Metric | Output | Description |
|--------|--------|-------------|
| `total` | `145823` | Total tokens consumed |
| `input` | `98234` | Input tokens |
| `output` | `47589` | Output tokens |
| `count` | `42` | Request count |
| `burn-rate` | `2145.3` | Tokens per minute (5-min window) |
| `burn-rate-hour` | `128700` | Projected tokens per hour |
| `block-remaining` | `3h42m` | Time left in current billing block |
| `block-tokens` | `89234` | Tokens in current billing block |

**Performance target**: <100ms end-to-end (process start → file read → output).

**Implementation notes**:
- Skip BoltDB position tracking; use `ReadFrom(path, 0)` for full reads
- Consider in-memory position cache for repeated calls
- For sub-10ms latency, connect to daemon via Unix socket (see Feature 5)

---

### Feature 2: `token-monitor serve` — MCP Server Mode

**Purpose**: Run as a persistent MCP server so Claude Code can query token data as tools.

**Interface**:

```bash
# Start as MCP stdio server (for Claude Code)
token-monitor serve --stdio

# Start as HTTP server (for external tools)
token-monitor serve --http :8080
```

**MCP Tools Provided**:

| Tool Name | Parameters | Returns |
|-----------|-----------|---------|
| `get_token_usage` | `session_id?` (auto-detect if omitted) | `{total, input, output, count, avg}` |
| `get_burn_rate` | `session_id?`, `window?` (default 5m) | `{tokens_per_min, tokens_per_hour, entries}` |
| `get_billing_block` | `session_id?` | `{start, end, tokens, time_remaining}` |
| `list_sessions` | `limit?` (default 10), `sort?` | `[{id, project, total_tokens, last_active}]` |
| `get_session_detail` | `session_id` | `{stats, burn_rate, billing_block, timeline}` |
| `compare_sessions` | `session_a`, `session_b` | `{diff_stats, a_stats, b_stats}` |

**MCP Configuration** (`.mcp.json`):

```json
{
  "mcpServers": {
    "token-monitor": {
      "command": "token-monitor",
      "args": ["serve", "--stdio"],
      "description": "Real-time Claude Code token usage monitoring"
    }
  }
}
```

**Example interaction inside Claude Code**:

> User: "How many tokens have I used in this session?"
> Claude: (calls `get_token_usage` tool) → "You've used 145,823 tokens so far — 98,234 input and 47,589 output. Current burn rate is 2,145 tokens/min."

**Implementation notes**:
- Use the MCP SDK for Go or implement the JSON-RPC 2.0 protocol over stdio
- Keep the monitor running in-process for real-time data
- File watcher stays active for live updates between tool calls

---

### Feature 3: `token-monitor status` — Status Line Output

**Purpose**: Produce a compact, formatted string for Claude Code's inline status display.

**Interface**:

```bash
# Default compact format
token-monitor status --current
# → 🔥 12.5K tokens | 2.1K/min | Block: 3h42m

# Minimal format (for narrow displays)
token-monitor status --current --compact
# → 12.5K/2.1K↑

# Full format
token-monitor status --current --full
# → Total: 12,534 | Rate: 2,145/min | In: 8,234 Out: 4,300 | Block: 3h42m left

# No emoji
token-monitor status --current --no-emoji
# → 12.5K tokens | 2.1K/min | Block: 3h42m

# Watch mode (continuous output, one line per interval)
token-monitor status --current --watch --interval 5s
```

**Format specifications**:

| Format | Example | Width |
|--------|---------|-------|
| `--compact` | `12.5K/2.1K↑` | ~13 chars |
| (default) | `🔥 12.5K tokens \| 2.1K/min \| Block: 3h42m` | ~45 chars |
| `--full` | `Total: 12,534 \| Rate: 2,145/min \| In: 8,234 Out: 4,300 \| Block: 3h42m left` | ~75 chars |

**Number formatting**:
- <1,000: exact (`823`)
- 1,000–999,999: K with 1 decimal (`12.5K`)
- 1,000,000+: M with 1 decimal (`1.2M`)

**Status Line Hook integration** (settings.json):

```json
{
  "hooks": {
    "PostToolUse": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "token-monitor status --current --compact 2>/dev/null || true"
      }]
    }]
  }
}
```

---

### Feature 4: Session Auto-Detection (`--current`)

**Purpose**: Foundation for all integration features. Automatically find the active Claude Code session without manual ID specification.

**Detection priority**:

```
1. Environment variable: CLAUDE_SESSION_ID
   └─ Set by Claude Code or injected by hooks

2. Environment variable: CLAUDE_PROJECT_DIR
   └─ Find most recently modified .jsonl in that project's directory

3. Default Claude config directories
   └─ Scan all projects, return the most recently modified .jsonl file
```

**Implementation**:

```go
// pkg/discovery/current.go

// FindCurrentSession returns the most recently active session.
// Detection priority:
//   1. CLAUDE_SESSION_ID env var
//   2. CLAUDE_PROJECT_DIR env var → most recent .jsonl
//   3. Default dirs → most recently modified .jsonl
func (d *discoverer) FindCurrentSession() (*SessionFile, error)
```

**Behavior**:
- If `CLAUDE_SESSION_ID` is set, find the matching session file directly
- If `CLAUDE_PROJECT_DIR` is set, scan that project directory only
- Otherwise, scan all configured directories and return the session with the most recent `ModTime`
- Cache the result for 1 second to avoid repeated filesystem scans

---

### Feature 5: Daemon Mode (Performance Optimization)

**Purpose**: Eliminate process startup overhead for repeated queries. Keep aggregated data in memory.

**Interface**:

```bash
# Start daemon
token-monitor daemon start
# → Daemon started, listening on /tmp/token-monitor.sock

# Query via daemon (fast — no file re-reading)
token-monitor query --current --metric total
# → (internally connects to /tmp/token-monitor.sock, gets response in <10ms)

# Stop daemon
token-monitor daemon stop

# Status
token-monitor daemon status
# → Running (PID 12345, 3 sessions monitored, uptime 2h15m)
```

**Architecture**:

```
┌──────────────────┐     Unix Socket      ┌────────────────────┐
│  query/status    │ ──────────────────▶   │     Daemon         │
│  (client mode)   │ ◀──────────────────   │                    │
│  <10ms response  │     JSON response     │  ┌──────────────┐  │
└──────────────────┘                       │  │   Monitor     │  │
                                           │  │  (in-memory)  │  │
┌──────────────────┐     Unix Socket      │  ├──────────────┤  │
│  serve --stdio   │ ──────────────────▶   │  │  Aggregator   │  │
│  (MCP server)    │ ◀──────────────────   │  │  (cached)     │  │
└──────────────────┘     JSON response     │  ├──────────────┤  │
                                           │  │  Watcher      │  │
                                           │  │  (fsnotify)   │  │
                                           │  └──────────────┘  │
                                           └────────────────────┘
```

**Behavior**:
- `query` and `status` commands check for a running daemon first
- If daemon is running: connect via Unix socket, get cached result (<10ms)
- If daemon is not running: fall back to direct file read (<100ms)
- Daemon auto-starts on first `query --current` call (opt-in via config)

---

## Hook Configuration Examples

### PostToolUse: Log Token Usage After Every Command

```json
{
  "hooks": {
    "PostToolUse": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "token-monitor query --current --format hook 2>/dev/null || true"
      }]
    }]
  }
}
```

### Stop: Session Summary on Exit

```json
{
  "hooks": {
    "Stop": [{
      "hooks": [{
        "type": "command",
        "command": "echo \"Session token usage:\"; token-monitor query --current --json 2>/dev/null || true"
      }]
    }]
  }
}
```

### SessionStart: Initialize Monitoring

```json
{
  "hooks": {
    "SessionStart": [{
      "hooks": [{
        "type": "command",
        "command": "token-monitor daemon start --quiet 2>/dev/null || true"
      }]
    }]
  }
}
```

---

## Skill/Command Template

A Claude Code skill that provides `/tokens` command:

```markdown
---
name: tokens
description: Show current session token usage and burn rate
---

Run `token-monitor query --current --json` and present the results:
- Total tokens (input + output breakdown)
- Burn rate (tokens/min and projected hourly)
- Current billing block status and time remaining
- Comparison to session average if available

Format the output in a clear, readable way for the user.
```

---

## Implementation Phases

### Phase 1: Quick Integration (query + status + auto-detect)

| Task | Package | Priority |
|------|---------|----------|
| Session auto-detection (`--current`) | `pkg/discovery` | High |
| `query` command with metric flags | `cmd/token-monitor` | High |
| `status` command with format options | `cmd/token-monitor` | High |
| Human-friendly number formatting (K/M) | `pkg/display` | Medium |
| Example hook configurations | `docs/` | Medium |

**Estimated new/modified files**:
- `pkg/discovery/current.go` — auto-detection logic
- `cmd/token-monitor/query_command.go` — query command
- `cmd/token-monitor/status_command.go` — status command
- `cmd/token-monitor/main.go` — register new commands

### Phase 2: Deep Integration (MCP Server)

| Task | Package | Priority |
|------|---------|----------|
| MCP JSON-RPC protocol handler | `pkg/mcp` | High |
| `serve --stdio` command | `cmd/token-monitor` | High |
| Tool definitions (get_token_usage, etc.) | `pkg/mcp/tools` | High |
| `.mcp.json` generator (`token-monitor setup mcp`) | `cmd/token-monitor` | Medium |
| Skill template for `/tokens` | `docs/` | Low |

**Estimated new files**:
- `pkg/mcp/server.go` — MCP protocol handler
- `pkg/mcp/tools.go` — tool definitions and handlers
- `cmd/token-monitor/serve_command.go` — serve command

### Phase 3: Performance (Daemon Mode)

| Task | Package | Priority |
|------|---------|----------|
| Daemon process management (start/stop) | `pkg/daemon` | High |
| Unix socket server | `pkg/daemon` | High |
| Client-side socket detection and fallback | `cmd/token-monitor` | High |
| Auto-start configuration | `pkg/config` | Medium |
| PID file management | `pkg/daemon` | Medium |

**Estimated new files**:
- `pkg/daemon/daemon.go` — background process management
- `pkg/daemon/socket.go` — Unix socket server/client
- `cmd/token-monitor/daemon_command.go` — daemon command

---

## Configuration Additions

```yaml
# token-monitor.yaml additions

# Integration settings
integration:
  # Auto-detect current session
  auto_detect: true

  # Daemon mode
  daemon:
    enabled: false
    socket_path: /tmp/token-monitor.sock
    auto_start: false

  # MCP server
  mcp:
    enabled: false

  # Status line format
  status:
    format: default  # compact | default | full
    emoji: true
```

---

## Security Considerations

- `serve` and `daemon` modes listen only on local interfaces (Unix socket or localhost)
- No authentication required for local-only access
- File permissions on Unix socket: `0600` (owner-only)
- MCP stdio mode inherits Claude Code's process permissions
- No sensitive data (API keys, secrets) is exposed through any interface
- Token counts are read-only; no write operations on Claude Code session files
