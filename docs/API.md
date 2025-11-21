# Token Monitor - API Reference

Complete reference for all public packages and interfaces.

---

## Table of Contents

- [Core Interfaces](#core-interfaces)
- [pkg/aggregator](#pkgaggregator)
- [pkg/config](#pkgconfig)
- [pkg/discovery](#pkgdiscovery)
- [pkg/display](#pkgdisplay)
- [pkg/logger](#pkglogger)
- [pkg/monitor](#pkgmonitor)
- [pkg/parser](#pkgparser)
- [pkg/reader](#pkgreader)
- [pkg/session](#pkgsession)
- [pkg/watcher](#pkgwatcher)

---

## Core Interfaces

### Aggregator

Computes token usage statistics across sessions, models, and time windows.

```go
type Aggregator interface {
    Add(entry parser.UsageEntry)
    Stats() Stats
    GroupedStats() map[string]Stats
    TopSessions(n int) []SessionStat
}
```

### Parser

Parses Claude Code JSONL usage files.

```go
type Parser interface {
    ParseFile(path string, offset int64) ([]UsageEntry, int64, error)
    ParseLine(line string) (*UsageEntry, error)
}
```

### Reader

Provides incremental reading of JSONL files with position tracking.

```go
type Reader interface {
    Read(ctx context.Context, path string) ([]parser.UsageEntry, error)
    ReadFrom(ctx context.Context, path string, offset int64) ([]parser.UsageEntry, int64, error)
    Reset(path string) error
    Close() error
}
```

### Watcher

Monitors filesystem for changes to session files.

```go
type Watcher interface {
    Start(paths []string) error
    Stop() error
    Events() <-chan Event
}
```

---

## pkg/aggregator

### Creating an Aggregator

```go
import "github.com/yourusername/token-monitor/pkg/aggregator"

agg := aggregator.New(aggregator.Config{
    GroupBy: []aggregator.Dimension{
        aggregator.DimModel,
        aggregator.DimSession,
    },
    TrackPercentiles: true,
})
```

### Configuration

**Type**: `Config`

```go
type Config struct {
    GroupBy          []Dimension  // Dimensions to group by
    TrackPercentiles bool         // Track P50, P95, P99
}
```

### Dimensions

Aggregation dimensions for grouping statistics.

```go
const (
    DimModel   Dimension = "model"    // Group by model name
    DimSession Dimension = "session"  // Group by session ID
    DimDate    Dimension = "date"     // Group by date (YYYY-MM-DD)
    DimHour    Dimension = "hour"     // Group by hour (YYYY-MM-DD HH:00)
)
```

### Data Structures

**Stats**: Overall token usage statistics

```go
type Stats struct {
    Count        int64      // Number of entries
    InputTokens  int64      // Total input tokens
    OutputTokens int64      // Total output tokens
    TotalTokens  int64      // Sum of all token types
    AvgTokens    float64    // Average tokens per entry
    MinTokens    int        // Minimum tokens in single entry
    MaxTokens    int        // Maximum tokens in single entry
    P50Tokens    int        // 50th percentile
    P95Tokens    int        // 95th percentile
    P99Tokens    int        // 99th percentile
    FirstSeen    time.Time  // Timestamp of first entry
    LastSeen     time.Time  // Timestamp of last entry

    // Cache-specific tokens
    CacheCreationTokens int64
    CacheReadTokens     int64
}
```

**SessionStat**: Per-session statistics with ranking

```go
type SessionStat struct {
    SessionID   string
    Stats       Stats
    Rank        int     // Rank by total tokens
}
```

**BurnRate**: Token consumption rate metrics

```go
type BurnRate struct {
    TokensPerMinute        float64
    TokensPerHour          float64
    InputTokensPerMinute   float64
    OutputTokensPerMinute  float64
    EntryCount             int
    WindowStart            time.Time
    WindowEnd              time.Time
}
```

**BillingBlock**: 5-hour UTC billing window information

```go
type BillingBlock struct {
    StartTime    time.Time
    EndTime      time.Time
    TotalTokens  int64
    InputTokens  int64
    OutputTokens int64
    EntryCount   int
}
```

### Methods

**Add**: Add a usage entry to the aggregator

```go
func (a *Aggregator) Add(entry parser.UsageEntry)
```

**Stats**: Get overall statistics

```go
func (a *Aggregator) Stats() Stats
```

**GroupedStats**: Get statistics grouped by configured dimensions

```go
func (a *Aggregator) GroupedStats() map[string]Stats
```

**TopSessions**: Get top N sessions by token usage

```go
func (a *Aggregator) TopSessions(n int) []SessionStat
```

**CalculateBurnRate**: Calculate token consumption rate

```go
func (a *Aggregator) CalculateBurnRate(windowMinutes int) BurnRate
```

**GetCurrentBlock**: Get current 5-hour billing block

```go
func (a *Aggregator) GetCurrentBlock() BillingBlock
```

---

## pkg/config

### Loading Configuration

```go
import "github.com/yourusername/token-monitor/pkg/config"

cfg, err := config.Load()
if err != nil {
    log.Fatal(err)
}
```

### Configuration Structure

```go
type Config struct {
    ClaudeConfigDirs []string       // Claude data directories
    Storage          StorageConfig  // Storage settings
    Logging          LoggingConfig  // Logging settings
}

type StorageConfig struct {
    DBPath string  // Path to BoltDB database
}

type LoggingConfig struct {
    Level  string  // debug, info, warn, error
    Format string  // text, json
    Output string  // stdout, stderr, or file path
}
```

### Default Configuration

```go
// Default search paths (in order)
1. ./token-monitor.yaml
2. ~/.config/token-monitor/config.yaml
3. /etc/token-monitor/config.yaml

// Default Claude directories
~/.config/claude/projects
~/.claude/projects
```

### Environment Variables

Override configuration with environment variables:

- `CLAUDE_CONFIG_DIR`: Comma-separated Claude data directories
- `TOKEN_MONITOR_DB_PATH`: Database path
- `TOKEN_MONITOR_LOG_LEVEL`: Log level

---

## pkg/discovery

### Creating a Discovery Instance

```go
import "github.com/yourusername/token-monitor/pkg/discovery"

disc := discovery.New(
    []string{"~/.config/claude/projects"},
    logger,
)

sessions, err := disc.Discover()
```

### Data Structures

**SessionInfo**: Information about a discovered session

```go
type SessionInfo struct {
    SessionID   string    // UUID of the session
    FilePath    string    // Path to usage.jsonl
    ProjectPath string    // Project directory path
    LastModTime time.Time // Last modification time
}
```

### Methods

**Discover**: Find all session files

```go
func (d *Discovery) Discover() ([]SessionInfo, error)
```

---

## pkg/display

### Creating a Display Formatter

```go
import "github.com/yourusername/token-monitor/pkg/display"

formatter := display.New(display.Config{
    Format:          display.FormatTable,
    ShowPercentiles: true,
    ShowTimestamps:  true,
    Compact:         false,
})
```

### Display Formats

```go
const (
    FormatTable  Format = iota  // Table format (default)
    FormatJSON                  // JSON output
    FormatSimple                // Simple text format
)
```

### Configuration

```go
type Config struct {
    Format          Format  // Output format
    ShowPercentiles bool    // Show P50, P95, P99
    ShowTimestamps  bool    // Show first/last seen
    Compact         bool    // Compact table output
}
```

### Methods

**FormatStats**: Format overall statistics

```go
func (f *Formatter) FormatStats(w io.Writer, stats aggregator.Stats) error
```

**FormatGroupedStats**: Format grouped statistics

```go
func (f *Formatter) FormatGroupedStats(
    w io.Writer,
    grouped map[string]aggregator.Stats,
    dimensions []string,
) error
```

**FormatTopSessions**: Format top N sessions

```go
func (f *Formatter) FormatTopSessions(
    w io.Writer,
    sessions []aggregator.SessionStat,
) error
```

---

## pkg/logger

### Creating a Logger

```go
import "github.com/yourusername/token-monitor/pkg/logger"

log := logger.New(logger.Config{
    Level:  "info",
    Format: "text",
    Output: "stdout",
})
```

### Configuration

```go
type Config struct {
    Level  string  // debug, info, warn, error
    Format string  // text, json
    Output string  // stdout, stderr, or file path
}
```

### Logger Interface

```go
type Logger interface {
    Debug(msg string, keysAndValues ...interface{})
    Info(msg string, keysAndValues ...interface{})
    Warn(msg string, keysAndValues ...interface{})
    Error(msg string, keysAndValues ...interface{})
    With(keysAndValues ...interface{}) Logger
}
```

### Noop Logger

```go
// Create a no-op logger for testing
log := logger.Noop()
```

---

## pkg/monitor

### Creating a Monitor

```go
import "github.com/yourusername/token-monitor/pkg/monitor"

mon, err := monitor.New(monitor.Config{
    SessionIDs:      []string{"abc123"},
    RefreshInterval: time.Second,
    ClearScreen:     true,
}, watcher, reader, discovery, logger)
```

### Configuration

```go
type Config struct {
    SessionIDs      []string      // Session IDs to monitor (empty = all)
    RefreshInterval time.Duration // Update frequency
    ClearScreen     bool          // Clear screen between updates
}
```

### Data Structures

**Update**: Live monitoring update

```go
type Update struct {
    Timestamp    time.Time
    Stats        aggregator.Stats
    Delta        Delta          // Changes since last update
    Cumulative   Delta          // Changes since monitoring started
    BurnRate     aggregator.BurnRate
    CurrentBlock aggregator.BillingBlock
}
```

**Delta**: Token usage changes

```go
type Delta struct {
    NewEntries   int
    InputTokens  int64
    OutputTokens int64
    TotalTokens  int64
}
```

### Methods

**Start**: Start monitoring

```go
func (m *Monitor) Start() error
```

**Stop**: Stop monitoring

```go
func (m *Monitor) Stop() error
```

**Updates**: Receive updates channel

```go
func (m *Monitor) Updates() <-chan Update
```

**Stats**: Get current statistics

```go
func (m *Monitor) Stats() aggregator.Stats
```

---

## pkg/parser

### Creating a Parser

```go
import "github.com/yourusername/token-monitor/pkg/parser"

p := parser.New()
```

### Data Structures

**UsageEntry**: Claude Code usage log entry

```go
type UsageEntry struct {
    Timestamp time.Time `json:"timestamp"`
    SessionID string    `json:"sessionId"`
    Version   string    `json:"version"`
    CurrentDir string   `json:"cwd"`
    Message   Message   `json:"message"`
    CostUSD   *float64  `json:"costUSD,omitempty"`
    RequestID *string   `json:"requestId,omitempty"`
}
```

**Message**: API response message

```go
type Message struct {
    ID      string   `json:"id"`
    Model   string   `json:"model"`
    Usage   Usage    `json:"usage"`
    Content []Content `json:"content"`
}
```

**Usage**: Token usage breakdown

```go
type Usage struct {
    InputTokens              int `json:"input_tokens"`
    OutputTokens             int `json:"output_tokens"`
    CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
    CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

func (u *Usage) TotalTokens() int {
    return u.InputTokens + u.OutputTokens +
           u.CacheCreationInputTokens + u.CacheReadInputTokens
}
```

### Methods

**ParseFile**: Parse entire JSONL file

```go
func (p *Parser) ParseFile(path string, offset int64) ([]UsageEntry, int64, error)
```

**ParseLine**: Parse single JSONL line

```go
func (p *Parser) ParseLine(line string) (*UsageEntry, error)
```

### Errors

```go
var (
    ErrInvalidTimestamp   = errors.New("invalid timestamp")
    ErrInvalidSessionID   = errors.New("invalid session ID")
    ErrInvalidModel       = errors.New("invalid model")
    ErrNegativeTokenCount = errors.New("invalid token count")
    ErrMalformedJSON      = errors.New("malformed JSON line")
    ErrFileTooLarge       = errors.New("file size exceeds maximum")
)
```

**ParseError**: Detailed parse error with context

```go
type ParseError struct {
    Line int    // Line number (1-indexed)
    Data string // Malformed line (truncated if too long)
    Err  error  // Underlying error
}
```

**ValidationError**: Validation failure context

```go
type ValidationError struct {
    Line      int    // Line number
    SessionID string // Session being validated
    Err       error  // Underlying error
}
```

---

## pkg/reader

### Creating a Reader

```go
import "github.com/yourusername/token-monitor/pkg/reader"

// Create position store
store := reader.NewMemoryPositionStore()
// or
store, err := reader.NewBoltPositionStore(db)

// Create reader
r, err := reader.New(reader.Config{
    PositionStore: store,
    Parser:        parser.New(),
}, logger)
```

### Configuration

```go
type Config struct {
    PositionStore PositionStore  // Position tracking
    Parser        parser.Parser  // JSONL parser
}
```

### Position Store

**PositionStore**: Tracks file read positions

```go
type PositionStore interface {
    GetPosition(path string) (int64, error)
    SetPosition(path string, offset int64) error
}
```

**Memory Position Store**: In-memory storage (for testing)

```go
func NewMemoryPositionStore() PositionStore
```

**Bolt Position Store**: Persistent BoltDB storage

```go
func NewBoltPositionStore(db *bbolt.DB) (PositionStore, error)
```

### Methods

**Read**: Read from last position

```go
func (r *Reader) Read(ctx context.Context, path string) ([]parser.UsageEntry, error)
```

**ReadFrom**: Read from specific offset

```go
func (r *Reader) ReadFrom(
    ctx context.Context,
    path string,
    offset int64,
) ([]parser.UsageEntry, int64, error)
```

**Reset**: Reset position to beginning

```go
func (r *Reader) Reset(path string) error
```

**Close**: Close reader

```go
func (r *Reader) Close() error
```

### Errors

```go
var (
    ErrReaderClosed      = errors.New("reader is closed")
    ErrFileLocked        = errors.New("file is locked")
    ErrFileNotFound      = errors.New("file not found")
    ErrPermissionDenied  = errors.New("permission denied")
    ErrFileTooLarge      = errors.New("file too large")
    ErrInvalidOffset     = errors.New("invalid offset")
)
```

---

## pkg/session

### Creating a Session Manager

```go
import "github.com/yourusername/token-monitor/pkg/session"

mgr, err := session.New(session.Config{
    DBPath: "~/.config/token-monitor/sessions.db",
}, logger)
```

### Configuration

```go
type Config struct {
    DBPath string  // Path to BoltDB database
}
```

### Data Structures

**SessionMetadata**: Session information

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

### Methods

**SetName**: Assign friendly name to session

```go
func (m *Manager) SetName(uuid, name string) error
```

**GetByName**: Lookup session by name

```go
func (m *Manager) GetByName(name string) (*SessionMetadata, error)
```

**GetByUUID**: Lookup session by UUID

```go
func (m *Manager) GetByUUID(uuid string) (*SessionMetadata, error)
```

**List**: List all sessions

```go
func (m *Manager) List() ([]SessionMetadata, error)
```

**Delete**: Remove session metadata

```go
func (m *Manager) Delete(uuid string) error
```

**Close**: Close database connection

```go
func (m *Manager) Close() error
```

**DB**: Get underlying BoltDB instance

```go
func (m *Manager) DB() *bbolt.DB
```

### Errors

```go
var (
    ErrNameConflict   = errors.New("name already in use")
    ErrSessionNotFound = errors.New("session not found")
    ErrInvalidUUID    = errors.New("invalid UUID format")
)
```

---

## pkg/watcher

### Creating a Watcher

```go
import "github.com/yourusername/token-monitor/pkg/watcher"

w, err := watcher.New(watcher.Config{
    DebounceInterval: 100 * time.Millisecond,
}, logger)
```

### Configuration

```go
type Config struct {
    DebounceInterval time.Duration  // Debounce file events
}
```

### Data Structures

**Event**: Filesystem event

```go
type Event struct {
    Path      string    // File path
    SessionID string    // Extracted session ID
    Type      EventType // Event type
    Time      time.Time // Event timestamp
}

type EventType int

const (
    EventCreated EventType = iota
    EventModified
    EventDeleted
)
```

### Methods

**Start**: Start watching paths

```go
func (w *Watcher) Start(paths []string) error
```

**Stop**: Stop watching

```go
func (w *Watcher) Stop() error
```

**Events**: Receive event channel

```go
func (w *Watcher) Events() <-chan Event
```

**Close**: Close watcher

```go
func (w *Watcher) Close() error
```

---

## Usage Examples

### Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/yourusername/token-monitor/pkg/aggregator"
    "github.com/yourusername/token-monitor/pkg/config"
    "github.com/yourusername/token-monitor/pkg/discovery"
    "github.com/yourusername/token-monitor/pkg/logger"
    "github.com/yourusername/token-monitor/pkg/parser"
    "github.com/yourusername/token-monitor/pkg/reader"
    "github.com/yourusername/token-monitor/pkg/session"
)

func main() {
    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }

    // Create logger
    logger := logger.New(logger.Config{
        Level:  cfg.Logging.Level,
        Format: cfg.Logging.Format,
        Output: cfg.Logging.Output,
    })

    // Create session manager
    sessionMgr, err := session.New(session.Config{
        DBPath: cfg.Storage.DBPath,
    }, logger)
    if err != nil {
        log.Fatal(err)
    }
    defer sessionMgr.Close()

    // Create reader with position store
    posStore, err := reader.NewBoltPositionStore(sessionMgr.DB())
    if err != nil {
        log.Fatal(err)
    }

    r, err := reader.New(reader.Config{
        PositionStore: posStore,
        Parser:        parser.New(),
    }, logger)
    if err != nil {
        log.Fatal(err)
    }
    defer r.Close()

    // Discover sessions
    disc := discovery.New(cfg.ClaudeConfigDirs, logger)
    sessions, err := disc.Discover()
    if err != nil {
        log.Fatal(err)
    }

    // Create aggregator
    agg := aggregator.New(aggregator.Config{
        GroupBy: []aggregator.Dimension{
            aggregator.DimModel,
        },
        TrackPercentiles: true,
    })

    // Read and aggregate data
    ctx := context.Background()
    for _, sess := range sessions {
        entries, readErr := r.Read(ctx, sess.FilePath)
        if readErr != nil {
            logger.Warn("failed to read session",
                "session", sess.SessionID,
                "error", readErr)
            continue
        }

        for _, entry := range entries {
            agg.Add(entry)
        }
    }

    // Get statistics
    stats := agg.Stats()
    fmt.Printf("Total tokens: %d\n", stats.TotalTokens)
    fmt.Printf("Sessions: %d\n", len(sessions))

    // Calculate burn rate
    burnRate := agg.CalculateBurnRate(5)
    fmt.Printf("Tokens/min: %.1f\n", burnRate.TokensPerMinute)
}
```

---

## Best Practices

### Error Handling

Always handle errors explicitly:

```go
entries, err := parser.ParseFile(path, 0)
if err != nil {
    return fmt.Errorf("parse failed: %w", err)
}
```

### Resource Cleanup

Use defer for cleanup:

```go
r, err := reader.New(config, logger)
if err != nil {
    return err
}
defer r.Close()
```

### Context Usage

Use context for cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

entries, err := reader.Read(ctx, path)
```

### Concurrent Access

Most types are safe for concurrent reads but not concurrent writes. Use appropriate synchronization:

```go
// Aggregator is thread-safe
var mu sync.Mutex
mu.Lock()
agg.Add(entry)
mu.Unlock()
```
