# Token Monitor Architecture

## Overview

Token Monitor is a real-time monitoring tool for Claude Code CLI token usage, written in Go. It tracks input/output tokens per session with live updates and persistent storage, exposes a JSON-RPC MCP server for Claude Code sub-agents, and ships an installer that wires itself into Claude Code's statusline, MCP registry, and PostToolUse hooks.

## Related Docs

- [README.md](../README.md) — User-facing overview, installation, usage
- [CHANGELOG.md](../CHANGELOG.md) — Release notes
- [TESTING.md](TESTING.md) — Testing strategy and coverage
- [roadmap.md](roadmap.md) — Future work

## Design Goals

1. **Real-time Monitoring**: Track token usage as it happens with minimal latency (<100ms update cycle)
2. **Session-Based Tracking**: Monitor specific Claude Code sessions by name/ID
3. **Cross-Session Visibility**: Aggregate usage across every discovered session for a chosen window
4. **Persistent Storage**: Maintain historical data across tool restarts
5. **Low Overhead**: Minimal CPU and memory footprint
6. **Cross-Platform**: Support macOS, Linux, Windows
7. **Safe Integration**: Idempotent, atomic, sentinel-guarded patches to user config files

## Architecture Layers

```
┌──────────────────────────────────────────────────────────────────┐
│  CLI / TUI Interface                                             │
│  - Subcommands: tui, stats, list, watch, session, config,        │
│    query, status, serve, install                                 │
│  - Bubble Tea TUI dashboard (default when no subcommand given)   │
└────────┬─────────────────────────────────┬───────────────────────┘
         │                                 │
┌────────▼──────────────┐      ┌───────────▼─────────────────────┐
│  MCP Server           │      │  Statusline Bridge              │
│  JSON-RPC 2.0 / stdio │      │  status --from-stdin            │
│  9 tools (6 single-   │      │  status --breakdown             │
│  session + 3 cross-   │      │  Reads Claude Code envelope on  │
│  session breakdown)   │      │  stdin, prints one compact line │
└────────┬──────────────┘      └───────────┬─────────────────────┘
         │                                 │
┌────────▼─────────────────────────────────▼───────────────────────┐
│  Install Automation (pkg/installer)                              │
│  - statusline: marker-block patching of                          │
│    ~/.claude/statusline-command.sh                               │
│  - mcp: JSON edit of ~/.claude.json (global) or ./.mcp.json      │
│  - hook: PostToolUse entry in ~/.claude/settings.json            │
│  - Sentinel `_managed_by: "token-monitor"` for ownership         │
│  - Atomic write (rename), refuses symlinks, *.bak.* on every     │
│    mutation, json.Decoder.UseNumber() for int64 precision        │
└────────┬─────────────────────────────────────────────────────────┘
         │
┌────────▼─────────────────────────────────────────────────────────┐
│  Monitoring Engine                                               │
│  - File watcher (fsnotify) -> debounced WatchEvents              │
│  - JSONL parser (parser.UsageEntry, parser.Usage)                │
│  - Token aggregator (overall stats, grouped, top-N, burn rate,   │
│    billing blocks, model breakdown, time / glob filters)         │
└────────┬─────────────────────────────────────────────────────────┘
         │
┌────────▼─────────────────────────────────────────────────────────┐
│  Cross-Session SessionLoader (pkg/sessionloader)                 │
│  - Shared by status --breakdown and the MCP cross-session tools  │
│  - Discovers all sessions, reads each via a fresh Reader,        │
│    accumulates entries, logs+skips per-session read failures     │
└────────┬─────────────────────────────────────────────────────────┘
         │
┌────────▼─────────────────────────────────────────────────────────┐
│  Storage                                                         │
│  - BoltDB (sessions metadata, reader position store)             │
│  - JSONL files under ~/.config/claude/projects, ~/.claude/       │
│    projects/ (read-only — owned by Claude Code)                  │
└──────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. File Watcher (`pkg/watcher`)

**Responsibilities:**
- Monitor `~/.config/claude/projects/` and `~/.claude/projects/` directories
- Detect new JSONL entries in real-time using filesystem events
- Support custom paths via environment variable `CLAUDE_CONFIG_DIR`
- Handle file rotation and new session creation

**Implementation:**
- Use `fsnotify` library for cross-platform file watching
- Debounce file change events (100ms window) to batch updates
- Concurrent file processing with worker pool
- Context-aware shutdown

**Key Functions:**
```go
type Watcher interface {
    Start(ctx context.Context, paths []string) error
    Stop() error
    Events() <-chan WatchEvent
}

type WatchEvent struct {
    FilePath  string
    SessionID string
    EventType EventType // Created, Modified, Deleted
    Timestamp time.Time
}
```

### 2. JSONL Parser (`pkg/parser`)

**Responsibilities:**
- Parse Claude Code JSONL format
- Extract token usage metrics
- Validate and sanitize entries
- Handle malformed lines gracefully (log and skip)

**Data Schema:**
```go
type UsageEntry struct {
    Timestamp  time.Time `json:"timestamp"`
    SessionID  string    `json:"sessionId"`
    Version    string    `json:"version"`
    CurrentDir string    `json:"cwd"`
    Message    Message   `json:"message"`
    CostUSD    *float64  `json:"costUSD,omitempty"`
    RequestID  *string   `json:"requestId,omitempty"`
}

type Message struct {
    ID      string    `json:"id"`
    Model   string    `json:"model"`
    Usage   Usage     `json:"usage"`
    Content []Content `json:"content"`
}

type Usage struct {
    InputTokens              int `json:"input_tokens"`
    OutputTokens             int `json:"output_tokens"`
    CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
    CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}
```

`Usage.TotalTokens()` is the canonical sum used everywhere — adding a new token field only requires updating one method.

### 3. Reader (`pkg/reader`)

**Responsibilities:**
- Incremental file reads keyed by `(path -> last byte offset)`
- Re-parse only the new tail since the previous read
- Provide an in-memory fallback when BoltDB is locked (concurrent `serve`)

**Implementation:**
- `PositionStore` interface with two implementations:
  - `BoltPositionStore` (persistent, survives restarts)
  - In-memory fallback used when BoltDB is locked by another instance
- `ReadFrom(ctx, path, offset)` returns `(entries, newOffset, err)` so callers can persist offsets externally
- `Read(ctx, path)` is the convenience wrapper that uses the configured store

### 4. Session Loader (`pkg/sessionloader`)

**Responsibilities:**
- Centralize the "discover sessions, read each, accumulate entries" pattern shared by:
  - `status --breakdown` (CLI)
  - `get_today_usage`, `get_usage_by_window`, `get_session_breakdown` (MCP)
- Eliminate the prior 3-way duplication of reader plumbing

**Contract:**
```go
type ReaderFactory func() (reader.Reader, error)

func LoadEntries(
    ctx context.Context,
    sessions []discovery.SessionFile,
    factory ReaderFactory,
    log Logger,
) ([]parser.UsageEntry, error)
```

Per-session read failures are logged at Warn level and skipped — a single corrupt JSONL file should not poison cross-session aggregation. Each `LoadEntries` call uses a fresh `Reader` (via the factory) so position-store state stays isolated.

### 5. Token Aggregator (`pkg/aggregator`)

**Responsibilities:**
- Aggregate token counts by configurable dimensions (model, session, date, hour)
- Calculate overall stats, percentiles (P50/P95/P99), top-N sessions
- Compute burn rate over a sliding window
- Detect billing blocks (5-hour UTC windows)
- Cross-session breakdown by model with time / glob filters

**Data Structures:**
```go
type Statistics struct {
    Count               int
    SessionCount        int
    TotalTokens         int
    InputTokens         int
    OutputTokens        int
    CacheCreationTokens int
    CacheReadTokens     int
    AvgTokens           float64
    MinTokens           int
    MaxTokens           int
    P50Tokens, P95Tokens, P99Tokens int
    FirstSeen, LastSeen time.Time
}

type SessionStats struct {
    SessionID  string
    Model      string
    Statistics Statistics
}

type ModelBreakdown struct {
    Model        string // exact model string from JSONL
    InputTokens  int
    OutputTokens int
    CacheCreate  int
    CacheRead    int
    TotalTokens  int
    EntryCount   int
}

type BurnRate struct {
    TokensPerMinute       float64
    TokensPerHour         float64
    InputTokensPerMinute  float64
    OutputTokensPerMinute float64
    WindowDuration        time.Duration
    EntryCount            int
    TotalTokens           int
    ProjectedHourlyTokens int
}

type BillingBlock struct {
    StartTime    time.Time
    EndTime      time.Time
    TotalTokens  int
    InputTokens  int
    OutputTokens int
    EntryCount   int
    IsActive     bool
}
```

Token counts are `int` (not `int64`) — Go's default `int` is 64-bit on every supported platform and matches the parser's wire types, eliminating int↔int64 conversion noise.

**Key Functions:**
```go
// Per-aggregator
func New(cfg Config) Aggregator
func (a Aggregator) Add(entry parser.UsageEntry)
func (a Aggregator) Stats() Statistics
func (a Aggregator) GroupedStats() map[string]Statistics
func (a Aggregator) TopSessions(n int) []SessionStats
func (a Aggregator) BurnRate(sessionID string, window time.Duration) BurnRate
func (a Aggregator) BillingBlocks(sessionID string) []BillingBlock
func (a Aggregator) CurrentBillingBlock(sessionID string) BillingBlock

// Cross-session helpers (operate on []parser.UsageEntry directly)
func BreakdownByModel(entries []parser.UsageEntry) map[string]ModelBreakdown
func MatchModel(model, glob string) bool
func FilterByModelGlob(entries []parser.UsageEntry, glob string) []parser.UsageEntry
func FilterSince(entries []parser.UsageEntry, since time.Time) []parser.UsageEntry
```

`BreakdownByModel` skips entries whose model is empty or `<synthetic>`. `MatchModel` is case-insensitive on ASCII and supports `*` / `?` wildcards via `filepath.Match`. `FilterSince` treats `time.Time{}` as "include all" — `display.ParseWindow("all", ...)` returns the zero time so the cross-session pipeline works without a special case.

### 6. Session Manager (`pkg/session`)

**Responsibilities:**
- Map session UUIDs to user-friendly names
- Persist session metadata
- Query sessions by name or ID
- Handle session lifecycle

**Storage:**
- BoltDB for lightweight embedded storage
- Schema: `sessions` bucket with UUID keys

**Data Model:**
```go
type SessionMetadata struct {
    UUID        string
    Name        string
    ProjectPath string
    CreatedAt   time.Time
    UpdatedAt   time.Time
    Tags        []string
    Description string
}
```

**Key Functions:**
```go
func (sm SessionManager) SetName(uuid, name string) error
func (sm SessionManager) GetByName(name string) (*SessionMetadata, error)
func (sm SessionManager) GetByUUID(uuid string) (*SessionMetadata, error)
func (sm SessionManager) List() ([]SessionMetadata, error)
func (sm SessionManager) Delete(uuid string) error
```

### 7. Display Engine (`pkg/display`)

**Responsibilities:**
- Format token statistics for terminal output (table, JSON, simple)
- Provide reusable formatting helpers consumed by every CLI subcommand
- Parse user-facing time windows shared between `status` and the MCP `get_usage_by_window` tool

**Helpers:**
```go
// Number / duration formatting
func FormatCompact(n int) string         // 12345 -> "12.3K"
func FormatTokenCount(n int) string      // 12345 -> "12,345"
func FormatDuration(d time.Duration) string

// Window parsing — shared by CLI status and MCP tools
//
// "today" / ""    -> midnight in local timezone
// "all"           -> time.Time{} (zero — include everything)
// "Nd" / "Nh"     -> now - N*duration
func ParseWindow(s string, now time.Time) (time.Time, error)
```

Output formatters live alongside (`FormatTable`, `FormatJSON`, `FormatSimple`).

### 8. TUI Dashboard (`pkg/tui`)

**Responsibilities:**
- Default subcommand when `token-monitor` is run with no arguments
- Full-screen dashboard with real-time refresh
- Multi-tab navigation: Dashboard / Sessions / Stats, plus a help overlay

**Implementation:**
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — Elm Architecture (Model → Update → View)
- [Bubbles](https://github.com/charmbracelet/bubbles) — keybindings, viewport, list widgets
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — styling
- Alt-screen + mouse cell motion enabled
- Past / current / total split: TUI start time is the boundary so a long-running session shows what was already there vs. what's been generated since you opened the UI

**File map:**
```
pkg/tui/
  app.go         Model, initModel, New(opts), top-level Update/View
  dashboard.go   tab=Dashboard view (live monitor + summary)
  sessions.go    tab=Sessions view (list, detail, three-way split)
  stats.go       tab=Stats view (overall stats, top sessions)
  keys.go        KeyMap with key.NewBinding() for q, ?, tab, shift-tab,
                 r (refresh), 1/2/3, ↑/↓, enter, esc, pgup/pgdn
  help.go        modal help overlay
  styles.go      shared lipgloss styles
```

Entry point: `tui.New(tui.Options{SessionID, Refresh, LogLevel})`. Wired from `cmd/token-monitor/main.go::runTUICommand`.

### 9. MCP Server (`pkg/mcp`)

**Responsibilities:**
- Expose token-monitor data as JSON-RPC 2.0 tools over stdio
- Let Claude Code sub-agents query their own usage dynamically (e.g. parent agent dispatches sub-agents on different models and needs to attribute spend)
- Handle `initialize`, `tools/list`, `tools/call`, `ping`

**Tool inventory (9 total):**

Single-session (legacy, v0.1.1):
- `get_token_usage` — totals + averages for one session
- `get_burn_rate` — tokens/min over a window (default 5m)
- `get_billing_block` — current 5-hour block with time remaining
- `list_sessions` — discovered sessions sorted by recent modification
- `get_session_detail` — stats + burn + block in one call
- `compare_sessions` — diff between two sessions

Cross-session breakdown (new in v0.2.0):
- `get_session_breakdown` — per-model totals for one session (synthetic models excluded)
- `get_today_usage` — cross-session totals today, optional `model_glob` filter
- `get_usage_by_window` — arbitrary window (`today`, `all`, `Nd`, `Nh`) with optional `model_glob`

**Error code policy:**
```go
type ParamError struct{ Msg string }
func NewParamError(msg string) *ParamError
```

`handleToolsCall` uses `errors.As` to detect `*ParamError` and maps it to JSON-RPC `-32602 InvalidParams`. Everything else (filesystem, JSON marshaling, session-not-found by ID — actually that last one *is* a ParamError) maps to `-32603 InternalError`. This lets sub-agents distinguish "I called wrong" from "the server is broken" without parsing error messages.

**Reader plumbing:** the `sessionContext` struct holds a `discovery.Discoverer` and a `func() (reader.Reader, error)` factory. Cross-session handlers call `loadAllEntries(disc, factory, log)`, which delegates to `pkg/sessionloader` so the same code path serves the CLI breakdown.

### 10. Installer (`pkg/installer`)

**Responsibilities:**
- Idempotent, reversible installation of token-monitor into Claude Code
- Three independently invocable targets (statusline, MCP registry, PostToolUse hook), plus an `install all` aggregator

**Safety invariants:**

1. **Marker-based block identification** for shell scripts:
   ```
   # >>> token-monitor >>> (managed block, do not edit)
   ...
   # <<< token-monitor <<<
   ```
   `PatchMarkerBlock(content, body)` replaces the block in place if present, otherwise appends. Empty body removes the block.

2. **Sentinel-based ownership** for JSON entries:
   - MCP: `"_managed_by": "token-monitor"` inside the `mcpServers."token-monitor"` object
   - Hook: same key inside the inner `hooks[].command` entry

   Entries lacking the sentinel are treated as user-authored and never silently overwritten or removed — even if their command/args happen to match ours.

3. **Atomic write** (`atomicWriteFile`):
   - Refuses to write through a symlink (would silently rewrite a dotfiles target)
   - Preserves existing file's permission bits (e.g. won't widen a 0600 user secret to 0644)
   - Temp file in the same directory, fsync, `os.Rename` for POSIX atomicity

4. **`json.Decoder.UseNumber()`** when reading `~/.claude.json` — preserves int64 precision above 2^53. Without this, opaque numeric IDs Claude Code stores in unrelated keys would silently corrupt on re-marshal.

5. **Backup before write**: every mutation produces `*.bak.YYYYMMDD-HHMMSS` (collision-suffixed if two backups happen in the same second).

**Install targets:**

| Target     | File                                         | Identification mechanism                |
|------------|----------------------------------------------|------------------------------------------|
| statusline | `~/.claude/statusline-command.sh`            | Marker block (shell)                     |
| MCP        | `~/.claude.json` or `./.mcp.json`            | `_managed_by` sentinel inside JSON entry |
| hook       | `~/.claude/settings.json` (PostToolUse)      | `_managed_by` sentinel inside JSON entry |

**Subcommand surface:**
```
token-monitor install statusline [--dry-run|--print|--uninstall|--target PATH]
token-monitor install mcp        [--global|--project|--absolute|--uninstall|--dry-run]
token-monitor install hook       [--uninstall|--dry-run]
token-monitor install all        [--dry-run]
token-monitor install --uninstall-all
```

`install all` is fail-fast (a step's failure short-circuits), `--uninstall-all` is fail-soft (continue past errors so partial state still gets cleaned up to the maximum extent possible).

## Data Flow

### Real-time Monitoring Flow

```
1. Claude Code writes JSONL entry
   ↓
2. fsnotify detects file modification
   ↓
3. Watcher debounces and emits WatchEvent
   ↓
4. Reader reads only the new tail since last offset
   ↓
5. Parser converts new lines into []parser.UsageEntry
   ↓
6. Aggregator updates session statistics
   ↓
7. TUI / watch / stats refreshes display
   ↓
8. Position store (BoltDB) persists new offset
```

### Cross-Session Breakdown Flow (`status --from-stdin --breakdown`)

```
1. Claude Code's statusline pipes JSON envelope to stdin
   ↓
2. readStatuslineInput() drains stdin (envelope ignored under --breakdown,
   but draining prevents SIGPIPE on Claude Code's end)
   ↓
3. discovery.Discoverer.Discover() lists every session under
   the configured Claude config dirs
   ↓
4. sessionloader.LoadEntries() reads each session via a fresh Reader,
   logging+skipping per-session failures
   ↓
5. aggregator.FilterSince(entries, ParseWindow(window, now))
   ↓
6. aggregator.FilterByModelGlob(entries, modelGlob)
   ↓
7. aggregator.BreakdownByModel(entries)  // synthetic models excluded
   ↓
8. formatBreakdown(window, breakdown) -> "day:340K | son:128K | opus:212K"
   ↓
9. Single line written to stdout
```

### MCP Tool Call Flow

```
1. Claude Code spawns `token-monitor serve --stdio` and writes
   a JSON-RPC request line to stdin
   ↓
2. Server.Run() bufio.Scanner reads one line
   ↓
3. parseRequest() unmarshals into Request{ID, Method, Params}
   ↓
4. dispatch() routes by Method:
     initialize / tools/list / tools/call / ping / notifications/initialized
   ↓
5. handleToolsCall() unmarshals ToolCallParams,
   calls ToolRegistry.Call(name, args)
   ↓
6. The registered handler runs:
     - Single-session tools: sessionContext.aggregateSession(sf)
       → aggregator -> Stats / BurnRate / CurrentBillingBlock
     - Cross-session tools (v0.2): loadAllEntries(disc, factory, log)
       → sessionloader.LoadEntries
       → FilterSince / FilterByModelGlob / BreakdownByModel
   ↓
7. textResult() wraps the response as ToolCallResult{Content[Text]}
   ↓
8. errors.As(err, &ParamError) maps validation failures to -32602,
   everything else to -32603
   ↓
9. writeResponse() writes one JSON line to stdout
```

### Session Query Flow (single-session)

```
1. User requests session by name
   ↓
2. SessionManager looks up UUID
   ↓
3. Discovery resolves UUID -> SessionFile (JSONL path)
   ↓
4. Aggregator retrieves stats from cache (or rebuilds via Reader)
   ↓
5. Display formats and renders output
```

## Performance Considerations

### Optimization Strategies

1. **Incremental File Reading**
   - PositionStore tracks last read byte offset per file
   - Only the new tail since the previous read is parsed
   - In-memory fallback when BoltDB is locked by another instance (concurrent `serve`)

2. **Deduplication**
   - JSONL entries are treated as append-only; offset-based reads avoid re-processing
   - Synthetic-model entries (`model == "<synthetic>"` or empty) are filtered before breakdown

3. **Concurrent Processing**
   - Worker pool for parallel file processing
   - Channel-based communication for events
   - Context-aware cancellation for graceful shutdown

4. **Memory Management**
   - Aggregator stats are O(unique groups), not O(entries)
   - Percentile tracking is opt-in (`Config.TrackPercentiles`) — disable for large-volume aggregations
   - TUI uses Bubble Tea's diff-based render; avoids full-screen redraws

5. **I/O Optimization**
   - Atomic file writes use temp-then-rename, single fsync per mutation
   - JSON Decoder reuses buffers
   - Backup writes are O_EXCL to avoid clobbering concurrent backups

### Performance Targets

- **Latency**: <100ms from JSONL write to UI update
- **Memory**: <50MB baseline, <200MB with 100 active sessions
- **CPU**: <5% average, <20% during intensive operations
- **Disk I/O**: <10 IOPS sustained, <100 IOPS peak
- **Test Coverage**: >78% for core packages

## Configuration

### Configuration File (`~/.config/token-monitor/config.yaml`)

```yaml
# Data sources
claude_config_dirs:
  - ~/.config/claude/projects/
  - ~/.claude/projects/

# Monitoring
monitoring:
  watch_interval: 1s
  update_frequency: 1s
  session_retention: 720h  # 30 days

# Performance
performance:
  worker_pool_size: 5
  cache_size: 100
  batch_window: 100ms

# Display
display:
  default_mode: live      # live | compact | table | json
  color_enabled: true
  refresh_rate: 1s

# Storage
storage:
  db_path: ~/.config/token-monitor/sessions.db
  cache_dir: ~/.config/token-monitor/cache/

# Logging
logging:
  level: info             # debug | info | warn | error
  output: stderr          # stdout | stderr | file path
  format: text            # text | json

# Integration with Claude Code ecosystem
integration:
  auto_detect: true
  daemon:
    enabled: false
    socket_path: /tmp/token-monitor.sock
    auto_start: false
  mcp:
    enabled: false
  status:
    format: default       # compact | default | full
    emoji: true
```

### Environment Variables

- `CLAUDE_CONFIG_DIR`: Comma-separated custom paths
- `CLAUDE_SESSION_ID`: Pin auto-detection to a specific session
- `CLAUDE_PROJECT_DIR`: Bias auto-detection toward a project
- `TOKEN_MONITOR_CONFIG`: Custom config file path
- `TOKEN_MONITOR_DB`: Custom database path
- `TOKEN_MONITOR_LOG_LEVEL`: Logging level (debug, info, warn, error)

## Error Handling

### Error Categories

1. **File System Errors**
   - Missing directories → create with user prompt
   - Permission denied → log warning, skip file
   - File locked → retry with backoff; BoltDB-locked reads fall back to in-memory position store
   - Symlink writes → refused outright (would silently rewrite dotfile target)

2. **Parse Errors**
   - Malformed JSON → log error, skip line, continue
   - Missing required fields → use default values
   - Invalid timestamps → use file mtime

3. **Cross-Session Read Errors**
   - Per-session failures in `sessionloader.LoadEntries` are logged at Warn and skipped — one corrupt JSONL must not poison cross-session aggregation

4. **MCP Errors**
   - Invalid params (missing required arg, unknown session ID, malformed window) → `*ParamError` → JSON-RPC `-32602 InvalidParams`
   - Internal failures (filesystem, marshaling) → `-32603 InternalError`
   - Unknown method → `-32601 MethodNotFound`

5. **Install Errors**
   - User-authored entry detected (no `_managed_by` sentinel) → refuse to overwrite, point user at the conflicting file
   - Symlink write attempt → refuse, surface resolved target path
   - Partial `install all` failure → surface a recovery hint pointing at `install --uninstall-all`

### Recovery Strategies

- **Graceful Degradation**: Continue operating with reduced functionality
- **Automatic Retry**: Exponential backoff for transient failures
- **Backup-Then-Modify**: Every install mutation produces a timestamped backup before writing
- **Logging**: Structured logging with context for debugging

## Security Considerations

1. **Data Privacy**
   - No conversation content stored (only metadata and metrics)
   - Secure file permissions (0600 preserved on existing files; 0700 for directories)

2. **Input Validation**
   - Sanitize all file paths
   - Validate JSON schema strictly
   - Reject negative token counts and zero timestamps in `parser.Validate`
   - Limit file sizes (max 100MB per JSONL — `reader.Config.MaxFileSize`)
   - Prevent path traversal attacks

3. **Install Safety**
   - Refuse to write through symlinks
   - Sentinel-based ownership prevents silent overwrite of user-authored entries
   - Atomic rename pattern prevents truncated config files on crash
   - `json.Decoder.UseNumber()` prevents int64 precision loss in opaque user data

4. **Resource Limits**
   - Max sessions tracked: bounded by configured cache size
   - Max file size: 100MB (configurable)
   - Memory budget: configurable cache_size
   - Worker pool size: configurable

## Testing Strategy

### Unit Tests
- All parsers with valid/invalid inputs
- Aggregation logic with edge cases
- Session manager CRUD operations
- Display formatting functions
- Window parsing (today, all, Nd, Nh, edge cases)
- Marker-block patcher (install, replace, remove, malformed)
- MCP error code mapping (`*ParamError` → -32602)

### Integration Tests
- End-to-end file watching and parsing
- Multi-session concurrent updates
- Database persistence and recovery
- Configuration loading and validation
- Install lifecycle: fresh install → re-install → uninstall → fresh install
- Cross-session breakdown end-to-end (CLI + MCP parity)

### Performance Tests
- Benchmark file parsing (target: >10K lines/sec)
- Stress test with many concurrent sessions
- Memory leak detection (long-running)
- I/O throughput measurement

### Test Coverage Target
- Overall: >80%
- Critical paths: >95% (parser, aggregator, session manager, installer)

## Implementation Status

### Completed (v0.1.0)

- Real-time file watching with fsnotify
- JSONL parsing with validation
- Token aggregation by session
- Burn rate calculation (5-minute sliding window)
- Billing block detection (5-hour UTC windows)
- Session naming and metadata storage (BoltDB)
- Live monitoring with terminal updates
- Multiple output formats (table, JSON, simple)
- CLI commands: `stats`, `list`, `watch`, `session`, `config`
- Configuration management (YAML + env vars + CLI flags)
- Comprehensive unit tests (78%+ coverage)
- CI/CD pipeline (GitHub Actions + goreleaser)

### Completed (v0.1.1)

- `query` command — fast single-metric token lookup (<100ms, no BoltDB)
- `status` command — compact / default / full formats with `--watch` and `--no-emoji`
- `serve` command — MCP JSON-RPC 2.0 server over stdio (6 single-session tools)
- Session auto-detection (`--current`) with env-var priority and 1s cache
- K/M number formatting helpers (`FormatCompact`, `FormatTokenCount`)
- Bubble Tea TUI dashboard as default subcommand
- Session list filters (`--project`, `--from`, `--to`, `--min-tokens`)
- Session export (CSV, JSON, agent-forge)
- BoltDB-locked fallback to in-memory position store for concurrent `serve`

### Completed (v0.2.0)

- Cross-session breakdown — `status --breakdown` (CLI) and 3 MCP tools (`get_session_breakdown`, `get_today_usage`, `get_usage_by_window`)
- Stdin-aware statusline integration — `status --from-stdin` consumes Claude Code's JSON envelope
- Install automation — `token-monitor install all` and per-component subcommands (`statusline` / `mcp` / `hook`)
- Atomic config-file writes — temp-then-rename, symlink refusal, `json.Decoder.UseNumber()` for int64 precision
- Sentinel-based ownership — `_managed_by: "token-monitor"` for MCP and hook entries
- Marker-based idempotent shell-script patching for statusline
- ParamError → -32602 InvalidParams mapping (vs -32603 InternalError)
- Shared `pkg/sessionloader` package — eliminates 3-way duplication of "discover + read + accumulate"
- Bubble Tea TUI dashboard remains the default subcommand

### Planned

See [roadmap.md](roadmap.md) for the full plan. Highlights:

1. **Daemon mode & performance** (v0.4.0) — Unix-socket server for sub-10ms queries
2. **Distribution polish** (v0.3.0) — Homebrew tap, interactive session selection
3. **Cost analysis & alerting** (v0.5+) — pricing API integration, budget thresholds, webhook alerts
