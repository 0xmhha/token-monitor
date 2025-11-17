# Token Monitor Architecture

## Overview

Token Monitor is a real-time monitoring tool for Claude Code CLI token usage, written in Go. It tracks input/output tokens per session with live updates and persistent storage.

## Design Goals

1. **Real-time Monitoring**: Track token usage as it happens with minimal latency (<100ms update cycle)
2. **Session-Based Tracking**: Monitor specific Claude Code sessions by name/ID
3. **Persistent Storage**: Maintain historical data across tool restarts
4. **Low Overhead**: Minimal CPU and memory footprint
5. **Cross-Platform**: Support macOS, Linux, Windows

## Architecture Layers

```
┌─────────────────────────────────────────────────────────┐
│                     CLI Interface                        │
│  (Session selection, display formatting, commands)       │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│                  Monitoring Engine                       │
│  - File watcher (inotify/fsnotify)                      │
│  - JSONL parser                                          │
│  - Token aggregator                                      │
│  - Real-time updater                                     │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│                   Data Layer                             │
│  - JSONL reader (Claude Code format)                    │
│  - Session mapper (UUID → name mapping)                 │
│  - Token calculator                                      │
│  - Cache manager                                         │
└────────────────┬────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────┐
│              Storage & Persistence                       │
│  - Session metadata DB (BoltDB/SQLite)                  │
│  - JSONL cache (deduplication)                          │
│  - Configuration store                                   │
└──────────────────────────────────────────────────────────┘
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
- Track file modification times for incremental reads
- Concurrent file processing with worker pool (5 goroutines)

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
- Handle malformed lines gracefully

**Data Schema:**
```go
type UsageEntry struct {
    Timestamp   time.Time  `json:"timestamp"`
    SessionID   string     `json:"sessionId"`
    Version     string     `json:"version"`
    CurrentDir  string     `json:"cwd"`
    Message     Message    `json:"message"`
    CostUSD     *float64   `json:"costUSD,omitempty"`
    RequestID   *string    `json:"requestId,omitempty"`
}

type Message struct {
    ID      string       `json:"id"`
    Model   string       `json:"model"`
    Usage   TokenUsage   `json:"usage"`
    Content []Content    `json:"content"`
}

type TokenUsage struct {
    InputTokens              int `json:"input_tokens"`
    OutputTokens             int `json:"output_tokens"`
    CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
    CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}
```

**Key Functions:**
```go
func ParseJSONL(reader io.Reader) ([]UsageEntry, error)
func ParseLine(line string) (*UsageEntry, error)
func ValidateEntry(entry *UsageEntry) error
```

### 3. Token Aggregator (`pkg/aggregator`)

**Responsibilities:**
- Aggregate token counts by session
- Calculate total costs
- Track billing blocks (5-hour windows)
- Compute burn rates and projections

**Data Structures:**
```go
type SessionStats struct {
    SessionID       string
    SessionName     string    // User-defined friendly name
    ProjectPath     string
    StartTime       time.Time
    LastActivity    time.Time

    InputTokens              int64
    OutputTokens             int64
    CacheCreationTokens      int64
    CacheReadTokens          int64

    TotalCost       float64
    ModelsUsed      []string
    MessageCount    int

    CurrentBlock    *BillingBlock
    BurnRate        float64  // tokens/minute
}

type BillingBlock struct {
    StartTime    time.Time
    EndTime      time.Time
    IsActive     bool
    TokensUsed   int64
    CostUSD      float64
}
```

**Key Functions:**
```go
func NewAggregator() *Aggregator
func (a *Aggregator) ProcessEntry(entry *UsageEntry) error
func (a *Aggregator) GetSessionStats(sessionID string) (*SessionStats, error)
func (a *Aggregator) GetAllSessions() ([]SessionStats, error)
func (a *Aggregator) CalculateBurnRate(sessionID string, windowMinutes int) (float64, error)
```

### 4. Session Manager (`pkg/session`)

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
func (sm *SessionManager) SetName(uuid, name string) error
func (sm *SessionManager) GetByName(name string) (*SessionMetadata, error)
func (sm *SessionManager) GetByUUID(uuid string) (*SessionMetadata, error)
func (sm *SessionManager) List() ([]SessionMetadata, error)
func (sm *SessionManager) Delete(uuid string) error
```

### 5. Display Engine (`pkg/display`)

**Responsibilities:**
- Real-time terminal UI updates
- Table formatting for token stats
- Progress bars and burn rate indicators
- Color-coded output

**Implementation:**
- Use `bubbletea` for TUI framework
- Update frequency: 1 second (configurable)
- Support both interactive and non-interactive modes

**Display Modes:**
```go
type DisplayMode int

const (
    ModeLive       DisplayMode = iota  // Full-screen live dashboard
    ModeCompact                        // Single-line status
    ModeTable                          // Static table output
    ModeJSON                           // JSON output
)
```

**Key Functions:**
```go
func NewDisplay(mode DisplayMode) *Display
func (d *Display) Update(stats *SessionStats) error
func (d *Display) Render() string
func (d *Display) Start(ctx context.Context) error
```

## Data Flow

### Real-time Monitoring Flow

```
1. Claude Code writes JSONL entry
   ↓
2. fsnotify detects file modification
   ↓
3. Watcher emits WatchEvent
   ↓
4. Parser reads new lines from file
   ↓
5. Aggregator updates session statistics
   ↓
6. Display engine refreshes UI
   ↓
7. Session metadata persisted to BoltDB
```

### Session Query Flow

```
1. User requests session by name
   ↓
2. SessionManager looks up UUID
   ↓
3. Aggregator retrieves stats from cache
   ↓
4. Display formats and renders output
```

## Performance Considerations

### Optimization Strategies

1. **Incremental File Reading**
   - Track last read position per file
   - Only read new lines since last update
   - Use `bufio.Scanner` for efficient line reading

2. **Deduplication**
   - Maintain hash set of processed entry IDs
   - Prevent duplicate processing on file rewrites
   - Clear old hashes after 24-hour retention

3. **Concurrent Processing**
   - Worker pool for parallel file processing
   - Channel-based communication for events
   - Context-aware cancellation for graceful shutdown

4. **Memory Management**
   - LRU cache for session statistics (limit: 100 sessions)
   - Periodic cleanup of old entries
   - Stream processing for large JSONL files

5. **I/O Optimization**
   - Batch database writes (100ms window)
   - Memory-mapped file access for hot files
   - Async file operations where possible

### Performance Targets

- **Latency**: <100ms from JSONL write to UI update
- **Memory**: <50MB baseline, <200MB with 100 active sessions
- **CPU**: <5% average, <20% during intensive operations
- **Disk I/O**: <10 IOPS sustained, <100 IOPS peak

## Configuration

### Configuration File (`~/.config/token-monitor/config.yaml`)

```yaml
# Data sources
claude_config_dirs:
  - ~/.config/claude/projects/
  - ~/.claude/projects/

# Monitoring
watch_interval: 1s
update_frequency: 1s
session_retention: 720h  # 30 days

# Performance
worker_pool_size: 5
cache_size: 100
batch_window: 100ms

# Display
default_mode: live
color_enabled: true
refresh_rate: 1s

# Storage
db_path: ~/.config/token-monitor/sessions.db
cache_dir: ~/.config/token-monitor/cache/
```

### Environment Variables

- `CLAUDE_CONFIG_DIR`: Comma-separated custom paths
- `TOKEN_MONITOR_CONFIG`: Custom config file path
- `TOKEN_MONITOR_DB`: Custom database path
- `TOKEN_MONITOR_LOG_LEVEL`: Logging level (debug, info, warn, error)

## Error Handling

### Error Categories

1. **File System Errors**
   - Missing directories → create with user prompt
   - Permission denied → log warning, skip file
   - File locked → retry with exponential backoff

2. **Parse Errors**
   - Malformed JSON → log error, skip line, continue
   - Missing required fields → use default values
   - Invalid timestamps → use file mtime

3. **Database Errors**
   - DB locked → retry with backoff
   - Corruption → attempt recovery, fallback to new DB
   - Disk full → log critical, pause writes

4. **Display Errors**
   - Terminal too small → switch to compact mode
   - TTY unavailable → fallback to non-interactive mode

### Recovery Strategies

- **Graceful Degradation**: Continue operating with reduced functionality
- **Automatic Retry**: Exponential backoff for transient failures
- **Circuit Breaker**: Pause operations after repeated failures
- **Logging**: Structured logging with context for debugging

## Security Considerations

1. **Data Privacy**
   - No conversation content stored (only metadata and metrics)
   - Session metadata encrypted at rest (optional)
   - Secure file permissions (0600 for DB, 0700 for directories)

2. **Input Validation**
   - Sanitize all file paths
   - Validate JSON schema strictly
   - Limit file sizes (max 100MB per JSONL)
   - Prevent path traversal attacks

3. **Resource Limits**
   - Max sessions tracked: 1000
   - Max file size: 100MB
   - Max memory: 500MB hard limit
   - CPU throttling if >50% for >30s

## Testing Strategy

### Unit Tests
- All parsers with valid/invalid inputs
- Aggregation logic with edge cases
- Session manager CRUD operations
- Display formatting functions

### Integration Tests
- End-to-end file watching and parsing
- Multi-session concurrent updates
- Database persistence and recovery
- Configuration loading and validation

### Performance Tests
- Benchmark file parsing (target: >10K lines/sec)
- Stress test with 100 concurrent sessions
- Memory leak detection (24-hour run)
- I/O throughput measurement

### Test Coverage Target
- Overall: >80%
- Critical paths: >95% (parser, aggregator, session manager)

## Future Enhancements

1. **Web Dashboard**
   - HTTP server with REST API
   - WebSocket for real-time updates
   - Interactive charts and graphs

2. **Cost Analysis**
   - Integration with LiteLLM pricing
   - Budget tracking and alerts
   - Cost projection and forecasting

3. **Alerting**
   - Token limit warnings
   - Cost threshold notifications
   - Slack/Discord integrations

4. **Export**
   - CSV export for analysis
   - PDF reports
   - Prometheus metrics export

5. **Multi-User**
   - Team usage aggregation
   - User-based cost allocation
   - Shared session tracking
