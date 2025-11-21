# Token Monitor - Usage Guide

Complete reference for all commands, flags, and workflows.

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Commands](#commands)
  - [stats](#stats---display-statistics)
  - [list](#list---list-sessions)
  - [watch](#watch---live-monitoring)
  - [session](#session---session-management)
  - [config](#config---configuration-management)
- [Common Workflows](#common-workflows)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [FAQ](#faq)

---

## Installation

### Using Go Install
```bash
go install github.com/0xmhha/token-monitor/cmd/token-monitor@latest
```

### From Source
```bash
git clone https://github.com/0xmhha/token-monitor.git
cd token-monitor
go build -o token-monitor ./cmd/token-monitor
sudo mv token-monitor /usr/local/bin/
```

### Binary Download
Download pre-built binaries from the [releases page](https://github.com/0xmhha/token-monitor/releases).

---

## Quick Start

```bash
# List all discovered Claude Code sessions
token-monitor list

# Show aggregated statistics
token-monitor stats

# Live monitoring of all sessions
token-monitor watch

# Name a session for easier reference
token-monitor session name <uuid> my-project

# Live monitoring of a specific session
token-monitor watch --session my-project
```

---

## Commands

### `stats` - Display Statistics

Show aggregated token usage statistics across sessions.

#### Syntax
```bash
token-monitor stats [flags]
```

#### Flags
- `--session <id>` - Filter by session ID or name
- `--model <name>` - Filter by model name (e.g., claude-3-sonnet)
- `--group-by <dimensions>` - Group by: model, session, date, hour (comma-separated)
- `--top <n>` - Show top N sessions by token usage
- `--format <type>` - Output format: table (default), json, simple
- `--compact` - Compact table output

#### Examples

**Show overall statistics:**
```bash
token-monitor stats
```

**Group by model:**
```bash
token-monitor stats --group-by model
```

**Show top 10 sessions:**
```bash
token-monitor stats --top 10
```

**JSON output:**
```bash
token-monitor stats --format json
```

**Filter by specific session:**
```bash
token-monitor stats --session my-project
```

**Multiple groupings:**
```bash
token-monitor stats --group-by model,date
```

---

### `list` - List Sessions

List all discovered Claude Code sessions with metadata.

#### Syntax
```bash
token-monitor list
```

#### Output
Shows a table with:
- Session UUID (first 8 characters)
- Session name (if assigned)
- Project path
- Last activity timestamp

#### Examples

```bash
# List all sessions
token-monitor list
```

---

### `watch` - Live Monitoring

Monitor token usage in real-time with live updates.

#### Syntax
```bash
token-monitor watch [flags]
```

#### Flags
- `--session <id>` - Monitor specific session (by ID or name)
- `--refresh <duration>` - Refresh interval (default: 1s, e.g., 500ms, 2s)
- `--format <type>` - Display format: table (default), simple
- `--history` - Keep history of updates (append mode, no clear screen)

#### Features
- Real-time token usage updates
- Burn rate calculation (tokens/min, tokens/hour)
- Current billing block status (5-hour UTC windows)
- Automatic session discovery
- Delta tracking (cumulative and per-update changes)

#### Examples

**Monitor all active sessions:**
```bash
token-monitor watch
```

**Monitor specific session:**
```bash
token-monitor watch --session my-project
```

**Fast refresh rate:**
```bash
token-monitor watch --refresh 500ms
```

**Simple format:**
```bash
token-monitor watch --format simple
```

**Keep update history (append mode):**
```bash
token-monitor watch --history
```

#### Output Explanation

**Table Format:**
```
Session: my-project (abc12345-...)
Project: /Users/username/project

Tokens        Total      Session +   Now +
─────────────────────────────────────────
Input         125,432    +1,234      +123
Output        45,123     +456        +45
Cache Create  28,901     +289        +28
Cache Read    3,456      +34         +3
─────────────────────────────────────────
Total         202,912    +2,013      +199

Burn Rate:    1,245 tokens/min (74,700 tokens/hour)

Current Block: 2024-01-15 00:00 UTC - 05:00 UTC
Block Tokens:  89,234 / 500,000 (17.8%)
Time Elapsed:  2h 23m / 5h (47.7%)
```

- **Total**: Cumulative tokens since session start
- **Session +**: Change since monitoring started
- **Now +**: Change in last refresh interval
- **Burn Rate**: Current token consumption rate
- **Current Block**: Active 5-hour UTC billing window

---

### `session` - Session Management

Manage session metadata including names, viewing details, and deletion.

#### Subcommands

#### `session name` - Assign Friendly Name

Assign a human-readable name to a session UUID.

**Syntax:**
```bash
token-monitor session name <uuid> <name>
```

**Examples:**
```bash
# Name a session
token-monitor session name abc12345-6789-... my-project

# Session auto-created if doesn't exist
token-monitor session name new-uuid-here experimental-feature
```

**Features:**
- Names must be unique
- Auto-creates session if it doesn't exist
- Use names anywhere UUIDs are accepted

---

#### `session list` - List All Sessions

List all sessions with metadata and sorting options.

**Syntax:**
```bash
token-monitor session list [flags]
```

**Flags:**
- `--sort <field>` - Sort by: name (default), date, uuid
- `--all` - Show all sessions including unnamed

**Examples:**
```bash
# List all named sessions
token-monitor session list

# Sort by last update date
token-monitor session list --sort date

# Show all sessions including unnamed
token-monitor session list --all
```

---

#### `session show` - Show Session Details

Display detailed information about a specific session.

**Syntax:**
```bash
token-monitor session show <name|uuid>
```

**Examples:**
```bash
# Show by name
token-monitor session show my-project

# Show by UUID
token-monitor session show abc12345-6789-...
```

---

#### `session delete` - Delete Session Metadata

Remove session metadata from the database. JSONL data files are preserved.

**Syntax:**
```bash
token-monitor session delete <name|uuid> [flags]
```

**Flags:**
- `--force` - Skip confirmation prompt

**Examples:**
```bash
# Delete with confirmation
token-monitor session delete my-project

# Delete without confirmation
token-monitor session delete my-project --force
```

**Note:** This only removes metadata. Original JSONL files in Claude's data directories are never modified.

---

### `config` - Configuration Management

Manage token-monitor configuration settings.

#### Subcommands

#### `config show` - Display Configuration

Show current configuration with sources.

**Syntax:**
```bash
token-monitor config show [flags]
```

**Flags:**
- `--format <type>` - Output format: yaml (default), json

**Examples:**
```bash
# Show config in YAML
token-monitor config show

# Show config in JSON
token-monitor config show --format json
```

---

#### `config path` - Show Config File Paths

Display configuration file search paths and active config location.

**Syntax:**
```bash
token-monitor config path
```

**Output:**
```
Configuration file search paths (in order of precedence):

  1. ./token-monitor.yaml [not found]
  2. ~/.config/token-monitor/config.yaml [found]
  3. /etc/token-monitor/config.yaml [not found]

Active configuration: ~/.config/token-monitor/config.yaml
```

---

#### `config reset` - Reset to Defaults

Reset configuration to default values.

**Syntax:**
```bash
token-monitor config reset [flags]
```

**Flags:**
- `--force` - Skip confirmation prompt
- `--output <path>` - Output path for config file (default: ~/.config/token-monitor/config.yaml)

**Examples:**
```bash
# Reset with confirmation
token-monitor config reset

# Reset without confirmation
token-monitor config reset --force

# Reset to custom location
token-monitor config reset --output ./my-config.yaml
```

---

## Common Workflows

### First-Time Setup

```bash
# 1. List discovered sessions
token-monitor list

# 2. Name important sessions
token-monitor session name abc12345... main-project
token-monitor session name def67890... side-project

# 3. View aggregated stats
token-monitor stats

# 4. Start live monitoring
token-monitor watch --session main-project
```

### Daily Monitoring

```bash
# Quick stats check
token-monitor stats --top 5

# Live monitoring current work
token-monitor watch --session current-project --refresh 2s
```

### Weekly Review

```bash
# View all sessions sorted by date
token-monitor session list --sort date

# Get detailed stats grouped by model and date
token-monitor stats --group-by model,date --format json > weekly-report.json

# Check top consumers
token-monitor stats --top 10
```

### Cleanup Old Sessions

```bash
# List all sessions
token-monitor session list --all

# Delete old sessions (metadata only)
token-monitor session delete old-project-name
```

### Export and Analysis

```bash
# Export stats to JSON for analysis
token-monitor stats --format json > stats.json

# Export specific session stats
token-monitor stats --session my-project --format json > my-project-stats.json

# Get stats grouped by hour for time-based analysis
token-monitor stats --group-by hour --format json > hourly-usage.json
```

---

## Configuration

### Configuration File

Token Monitor searches for configuration files in this order:

1. `./token-monitor.yaml` (current directory)
2. `~/.config/token-monitor/config.yaml` (user config)
3. `/etc/token-monitor/config.yaml` (system config)

### Configuration Options

```yaml
# Claude Code data directories
claude_config_dirs:
  - ~/.config/claude/projects
  - ~/.claude/projects

# Storage settings
storage:
  db_path: ~/.local/share/token-monitor/sessions.db

# Logging settings
logging:
  level: info        # debug, info, warn, error
  format: text       # text, json
  output: stdout     # stdout, stderr, or file path
```

### Environment Variables

Override configuration with environment variables:

```bash
export CLAUDE_CONFIG_DIR="~/.config/claude/projects,/custom/path"
export TOKEN_MONITOR_DB_PATH="~/custom/sessions.db"
export TOKEN_MONITOR_LOG_LEVEL="debug"
```

### Command-Line Flags

Global flags override both config file and environment variables:

```bash
token-monitor --config ./custom-config.yaml stats
```

---

## Troubleshooting

### No Sessions Found

**Problem:** `token-monitor list` shows no sessions

**Solutions:**
1. Verify Claude Code is installed and has been used
2. Check Claude data directory exists:
   ```bash
   ls ~/.config/claude/projects
   ```
3. Set custom directory:
   ```bash
   export CLAUDE_CONFIG_DIR="/path/to/claude/data"
   token-monitor list
   ```

### Session Not Found

**Problem:** Cannot find session by name

**Solutions:**
1. List all sessions to verify name:
   ```bash
   token-monitor session list
   ```
2. Use UUID instead of name:
   ```bash
   token-monitor watch --session abc12345-...
   ```

### Live Monitoring Not Updating

**Problem:** `watch` command shows no updates

**Solutions:**
1. Verify Claude Code is actively running and creating entries
2. Check file permissions on JSONL files
3. Try increasing refresh interval:
   ```bash
   token-monitor watch --refresh 5s
   ```

### High Memory Usage

**Problem:** Token Monitor using excessive memory

**Solutions:**
1. Monitor specific sessions instead of all:
   ```bash
   token-monitor watch --session specific-name
   ```
2. Reduce refresh rate:
   ```bash
   token-monitor watch --refresh 5s
   ```

### Permission Denied

**Problem:** Cannot read session files

**Solutions:**
1. Check file permissions:
   ```bash
   ls -la ~/.config/claude/projects/*/usage.jsonl
   ```
2. Ensure you're the owner of the files
3. Run with appropriate permissions (avoid using sudo)

---

## FAQ

### What data does Token Monitor collect?

Token Monitor reads JSONL files created by Claude Code in `~/.config/claude/projects/`. It only reads token usage data and never modifies these files.

### Does it send data anywhere?

No. Token Monitor runs entirely locally and never sends data to external servers.

### Can I monitor multiple Claude Code instances?

Yes. Token Monitor discovers all sessions in the configured directories and can monitor them simultaneously.

### How accurate is the burn rate calculation?

Burn rate is calculated using a 5-minute sliding window of recent token usage. It provides a real-time estimate but may vary based on usage patterns.

### What are billing blocks?

Claude uses 5-hour UTC billing windows (00:00-05:00, 05:00-10:00, etc.). Token Monitor tracks your usage within these blocks to help manage limits.

### Can I export my data?

Yes. Use `--format json` with any command to get JSON output:
```bash
token-monitor stats --format json > export.json
```

### Does deleting a session delete my data?

No. `session delete` only removes the metadata (name, tags) from Token Monitor's database. Your original JSONL files in Claude's directories are never modified or deleted.

### How do I update Token Monitor?

```bash
# If installed via go install
go install github.com/0xmhha/token-monitor/cmd/token-monitor@latest

# Or download latest binary from releases
```

### Where is data stored?

- **Session metadata**: `~/.local/share/token-monitor/sessions.db` (BoltDB)
- **File positions**: Stored in same database
- **Configuration**: `~/.config/token-monitor/config.yaml`
- **Claude data**: Token Monitor only reads from `~/.config/claude/projects/`

### Can I use this with other LLM tools?

Currently Token Monitor is designed specifically for Claude Code's JSONL format. Support for other tools could be added in the future.

---

## Getting Help

- **Issues**: https://github.com/0xmhha/token-monitor/issues
- **Discussions**: https://github.com/0xmhha/token-monitor/discussions
- **Documentation**: https://github.com/0xmhha/token-monitor

---

## See Also

- [README.md](README.md) - Project overview and quick start
- [CONTRIBUTING.md](CONTRIBUTING.md) - Development guide
- [ARCHITECTURE.md](docs/ARCHITECTURE.md) - Technical architecture
- [CHANGELOG.md](CHANGELOG.md) - Version history
