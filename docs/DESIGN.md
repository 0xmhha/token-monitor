# Token Monitor Design Document

## Executive Summary

**Token Monitor** is a real-time monitoring tool for Claude Code CLI token usage, implemented in Go. It provides session-based tracking, live updates, and persistent storage for token consumption analysis.

**Key Design Decisions**:
- **Language**: Go (performance, concurrency, cross-platform)
- **Architecture**: Event-driven with file watching
- **Storage**: BoltDB (embedded, zero-config)
- **UI**: Bubbletea TUI framework
- **Data Source**: Claude Code JSONL logs (read-only)

---

## Design Principles

### 1. Minimal Overhead
- **Target**: <5% CPU usage, <50MB memory baseline
- **Strategy**: Incremental file reading, efficient caching, worker pools

### 2. Real-time Responsiveness
- **Target**: <100ms latency from JSONL write to UI update
- **Strategy**: fsnotify event-driven architecture, debounced updates

### 3. User-Friendly Interface
- **Target**: Intuitive CLI, beautiful terminal UI
- **Strategy**: Cobra commands, Bubbletea TUI, color-coded output

### 4. Data Integrity
- **Target**: Zero data loss, accurate token counts
- **Strategy**: Read-only access to source data, transactional DB writes, deduplication

### 5. Cross-Platform Compatibility
- **Target**: macOS, Linux, Windows support
- **Strategy**: Pure Go implementation, platform-agnostic libraries

---

## System Architecture

### High-Level Components

```
┌──────────────┐
│  Claude Code │ (External)
│     CLI      │
└──────┬───────┘
       │ writes JSONL
       ▼
┌─────────────────────────────────────────┐
│   ~/.config/claude/projects/            │
│   {projectDir}/{sessionId}.jsonl        │
└──────┬──────────────────────────────────┘
       │ monitored by
       ▼
┌─────────────────────────────────────────┐
│        Token Monitor (Go)               │
│                                         │
│  ┌──────────┐  ┌──────────┐  ┌────────┐│
│  │  Watcher │→ │  Parser  │→ │Aggreg- ││
│  │(fsnotify)│  │ (JSONL)  │  │ ator   ││
│  └──────────┘  └──────────┘  └───┬────┘│
│                                   │     │
│  ┌──────────┐  ┌──────────┐      │     │
│  │ Session  │← │  Display │ ←────┘     │
│  │ Manager  │  │(bubbletea)            │
│  │(BoltDB)  │  │          │            │
│  └──────────┘  └──────────┘            │
└─────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────┐
│    ~/.config/token-monitor/             │
│    - sessions.db (BoltDB)               │
│    - config.yaml                        │
│    - cache/                             │
└─────────────────────────────────────────┘
```

### Component Interactions

```
File Change Event Flow:
1. Claude Code writes JSONL entry
2. fsnotify detects file modification
3. Watcher emits event (debounced 100ms)
4. Parser reads new lines incrementally
5. Deduplicator checks hash
6. Aggregator updates session stats
7. Display refreshes UI (1s interval)
8. Session metadata persisted to BoltDB
```

---

## Data Models

### Core Data Structures

#### 1. JSONL Entry (from Claude Code)
```go
// Raw entry from Claude Code JSONL file
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

type Content struct {
    Type string  `json:"type"`
    Text *string `json:"text,omitempty"`
}
```

#### 2. Session Metadata (stored in BoltDB)
```go
// User-defined session metadata
type SessionMetadata struct {
    UUID        string    `json:"uuid"`
    Name        string    `json:"name"`          // User-friendly name
    ProjectPath string    `json:"projectPath"`
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
    Tags        []string  `json:"tags"`
    Description string    `json:"description"`
}
```

#### 3. Session Statistics (computed)
```go
// Aggregated token statistics per session
type SessionStats struct {
    SessionID       string    `json:"sessionId"`
    SessionName     string    `json:"sessionName"`
    ProjectPath     string    `json:"projectPath"`
    StartTime       time.Time `json:"startTime"`
    LastActivity    time.Time `json:"lastActivity"`

    // Token counts
    InputTokens              int64 `json:"inputTokens"`
    OutputTokens             int64 `json:"outputTokens"`
    CacheCreationTokens      int64 `json:"cacheCreationTokens"`
    CacheReadTokens          int64 `json:"cacheReadTokens"`
    TotalTokens              int64 `json:"totalTokens"`

    // Cost (future)
    TotalCost       float64  `json:"totalCost"`

    // Metadata
    ModelsUsed      []string `json:"modelsUsed"`
    MessageCount    int      `json:"messageCount"`

    // Real-time metrics
    CurrentBlock    *BillingBlock `json:"currentBlock,omitempty"`
    BurnRate        float64       `json:"burnRate"` // tokens/minute
}
```

#### 4. Billing Block
```go
// 5-hour UTC-based billing window
type BillingBlock struct {
    ID           string    `json:"id"`        // ISO timestamp of block start
    StartTime    time.Time `json:"startTime"` // UTC block start (00:00, 05:00, etc.)
    EndTime      time.Time `json:"endTime"`   // UTC block end
    IsActive     bool      `json:"isActive"`  // Within 5 hours of start
    IsGap        bool      `json:"isGap"`     // Inactivity gap

    // Usage within block
    TokensUsed   int64   `json:"tokensUsed"`
    CostUSD      float64 `json:"costUsd"`
    MessageCount int     `json:"messageCount"`
    Models       []string `json:"models"`
}
```

---

## Detailed Component Design

### 1. File Watcher (`pkg/watcher`)

**Purpose**: Monitor Claude data directories for JSONL file changes.

**Implementation**:
```go
package watcher

import (
    "context"
    "github.com/fsnotify/fsnotify"
)

type Watcher interface {
    Start(ctx context.Context, paths []string) error
    Stop() error
    Events() <-chan WatchEvent
}

type WatchEvent struct {
    FilePath  string
    SessionID string
    EventType EventType
    Timestamp time.Time
}

type EventType int

const (
    EventCreated EventType = iota
    EventModified
    EventDeleted
)

type fileWatcher struct {
    watcher   *fsnotify.Watcher
    events    chan WatchEvent
    debouncer *Debouncer
}

func New() (Watcher, error) {
    fw, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }

    return &fileWatcher{
        watcher:   fw,
        events:    make(chan WatchEvent, 100),
        debouncer: NewDebouncer(100 * time.Millisecond),
    }, nil
}

func (w *fileWatcher) Start(ctx context.Context, paths []string) error {
    for _, path := range paths {
        if err := w.watcher.Add(path); err != nil {
            return fmt.Errorf("failed to watch %s: %w", path, err)
        }
    }

    go w.eventLoop(ctx)
    return nil
}

func (w *fileWatcher) eventLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case event := <-w.watcher.Events:
            w.debouncer.Add(event.Name, func() {
                w.handleEvent(event)
            })
        case err := <-w.watcher.Errors:
            log.Error("watcher error", "error", err)
        }
    }
}
```

**Key Features**:
- **Debouncing**: 100ms window to batch rapid file changes
- **Recursive Watching**: Monitor all subdirectories
- **Error Recovery**: Automatic restart on watcher failures
- **Resource Cleanup**: Proper shutdown on context cancellation

---

### 2. JSONL Parser (`pkg/parser`)

**Purpose**: Parse Claude Code JSONL format and extract token usage.

**Implementation**:
```go
package parser

import (
    "bufio"
    "encoding/json"
    "io"
)

type Parser interface {
    ParseFile(path string, offset int64) ([]UsageEntry, int64, error)
    ParseLine(line string) (*UsageEntry, error)
}

type jsonlParser struct {
    validator *Validator
}

func New() Parser {
    return &jsonlParser{
        validator: NewValidator(),
    }
}

func (p *jsonlParser) ParseFile(path string, offset int64) ([]UsageEntry, int64, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, 0, err
    }
    defer f.Close()

    // Seek to offset for incremental reading
    if _, err := f.Seek(offset, io.SeekStart); err != nil {
        return nil, 0, err
    }

    entries := make([]UsageEntry, 0, 100)
    scanner := bufio.NewScanner(f)
    lineNum := 0

    for scanner.Scan() {
        lineNum++
        line := scanner.Text()

        entry, err := p.ParseLine(line)
        if err != nil {
            log.Warn("skipping malformed line", "line", lineNum, "error", err)
            continue
        }

        if err := p.validator.Validate(entry); err != nil {
            log.Warn("invalid entry", "line", lineNum, "error", err)
            continue
        }

        entries = append(entries, *entry)
    }

    // Get new offset
    newOffset, _ := f.Seek(0, io.SeekCurrent)

    return entries, newOffset, scanner.Err()
}

func (p *jsonlParser) ParseLine(line string) (*UsageEntry, error) {
    var entry UsageEntry
    if err := json.Unmarshal([]byte(line), &entry); err != nil {
        return nil, fmt.Errorf("json unmarshal: %w", err)
    }
    return &entry, nil
}
```

**Key Features**:
- **Incremental Parsing**: Only read new lines since last offset
- **Error Resilience**: Skip malformed lines, log warnings
- **Validation**: Ensure required fields present
- **Streaming**: Use bufio.Scanner for memory efficiency

---

### 3. Token Aggregator (`pkg/aggregator`)

**Purpose**: Aggregate token usage by session and compute statistics.

**Implementation**:
```go
package aggregator

type Aggregator interface {
    ProcessEntry(entry *UsageEntry) error
    GetSessionStats(sessionID string) (*SessionStats, error)
    GetAllSessions() ([]SessionStats, error)
    CalculateBurnRate(sessionID string, windowMinutes int) (float64, error)
}

type aggregator struct {
    cache      *lru.Cache           // sessionID → SessionStats
    dedup      *dedup.Deduplicator
    sessions   *session.Manager
    mu         sync.RWMutex
}

func New(sessions *session.Manager, cacheSize int) (Aggregator, error) {
    cache, err := lru.New(cacheSize)
    if err != nil {
        return nil, err
    }

    return &aggregator{
        cache:    cache,
        dedup:    dedup.New(24 * time.Hour),
        sessions: sessions,
    }, nil
}

func (a *aggregator) ProcessEntry(entry *UsageEntry) error {
    // Check deduplication
    entryHash := hashEntry(entry)
    if a.dedup.Contains(entryHash) {
        return nil // Already processed
    }
    a.dedup.Add(entryHash)

    a.mu.Lock()
    defer a.mu.Unlock()

    // Get or create session stats
    stats, err := a.getOrCreateStats(entry.SessionID)
    if err != nil {
        return err
    }

    // Update token counts
    stats.InputTokens += int64(entry.Message.Usage.InputTokens)
    stats.OutputTokens += int64(entry.Message.Usage.OutputTokens)
    stats.CacheCreationTokens += int64(entry.Message.Usage.CacheCreationInputTokens)
    stats.CacheReadTokens += int64(entry.Message.Usage.CacheReadInputTokens)
    stats.TotalTokens = stats.InputTokens + stats.OutputTokens +
        stats.CacheCreationTokens + stats.CacheReadTokens

    // Update metadata
    stats.LastActivity = entry.Timestamp
    stats.MessageCount++
    if !contains(stats.ModelsUsed, entry.Message.Model) {
        stats.ModelsUsed = append(stats.ModelsUsed, entry.Message.Model)
    }

    // Update billing block
    if err := a.updateBillingBlock(stats, entry); err != nil {
        return err
    }

    // Update cache
    a.cache.Add(entry.SessionID, stats)

    return nil
}

func (a *aggregator) updateBillingBlock(stats *SessionStats, entry *UsageEntry) error {
    blockStart := getBillingBlockStart(entry.Timestamp)
    blockEnd := blockStart.Add(5 * time.Hour)

    if stats.CurrentBlock == nil || stats.CurrentBlock.StartTime != blockStart {
        // New billing block
        stats.CurrentBlock = &BillingBlock{
            ID:        blockStart.Format(time.RFC3339),
            StartTime: blockStart,
            EndTime:   blockEnd,
            IsActive:  time.Now().UTC().Before(blockEnd),
        }
    }

    // Update block stats
    block := stats.CurrentBlock
    block.TokensUsed += int64(entry.Message.Usage.InputTokens +
        entry.Message.Usage.OutputTokens +
        entry.Message.Usage.CacheCreationInputTokens +
        entry.Message.Usage.CacheReadInputTokens)
    block.MessageCount++
    if !contains(block.Models, entry.Message.Model) {
        block.Models = append(block.Models, entry.Message.Model)
    }

    return nil
}

func getBillingBlockStart(t time.Time) time.Time {
    utc := t.UTC()
    hour := utc.Hour()
    blockHour := (hour / 5) * 5 // Round down to nearest 5-hour boundary
    return time.Date(utc.Year(), utc.Month(), utc.Day(), blockHour, 0, 0, 0, time.UTC)
}
```

**Key Features**:
- **LRU Cache**: Keep 100 most recent sessions in memory
- **Deduplication**: Hash-based duplicate detection
- **Billing Blocks**: Automatic 5-hour UTC block detection
- **Concurrency**: Thread-safe with RWMutex

---

### 4. Session Manager (`pkg/session`)

**Purpose**: Manage session metadata with UUID ↔ name mapping.

**Implementation**:
```go
package session

import (
    bolt "go.etcd.io/bbolt"
)

type Manager interface {
    SetName(uuid, name string) error
    GetByName(name string) (*SessionMetadata, error)
    GetByUUID(uuid string) (*SessionMetadata, error)
    List() ([]SessionMetadata, error)
    Delete(uuid string) error
}

type manager struct {
    db *bolt.DB
}

const (
    sessionsBucket = "sessions"
    nameIndexBucket = "name_index"
)

func New(dbPath string) (Manager, error) {
    db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
    if err != nil {
        return nil, err
    }

    // Initialize buckets
    if err := db.Update(func(tx *bolt.Tx) error {
        if _, err := tx.CreateBucketIfNotExists([]byte(sessionsBucket)); err != nil {
            return err
        }
        if _, err := tx.CreateBucketIfNotExists([]byte(nameIndexBucket)); err != nil {
            return err
        }
        return nil
    }); err != nil {
        return nil, err
    }

    return &manager{db: db}, nil
}

func (m *manager) SetName(uuid, name string) error {
    return m.db.Update(func(tx *bolt.Tx) error {
        sessions := tx.Bucket([]byte(sessionsBucket))
        nameIndex := tx.Bucket([]byte(nameIndexBucket))

        // Check name uniqueness
        if existing := nameIndex.Get([]byte(name)); existing != nil {
            existingUUID := string(existing)
            if existingUUID != uuid {
                return fmt.Errorf("name %q already used by session %s", name, existingUUID)
            }
        }

        // Get or create metadata
        var meta SessionMetadata
        if data := sessions.Get([]byte(uuid)); data != nil {
            if err := json.Unmarshal(data, &meta); err != nil {
                return err
            }
            // Remove old name from index if changed
            if meta.Name != "" && meta.Name != name {
                nameIndex.Delete([]byte(meta.Name))
            }
        } else {
            // New session
            meta = SessionMetadata{
                UUID:      uuid,
                CreatedAt: time.Now(),
            }
        }

        // Update metadata
        meta.Name = name
        meta.UpdatedAt = time.Now()

        // Serialize and store
        data, err := json.Marshal(meta)
        if err != nil {
            return err
        }

        if err := sessions.Put([]byte(uuid), data); err != nil {
            return err
        }

        // Update name index
        return nameIndex.Put([]byte(name), []byte(uuid))
    })
}
```

**Key Features**:
- **Embedded Database**: BoltDB (zero external dependencies)
- **Dual Indexing**: UUID primary key, name unique index
- **ACID Transactions**: Atomic updates
- **Efficient Lookups**: O(log n) for both UUID and name

---

### 5. Display Engine (`pkg/display`)

**Purpose**: Render real-time terminal UI with token statistics.

**Implementation**:
```go
package display

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

type Model struct {
    mode       DisplayMode
    stats      *SessionStats
    width      int
    height     int
    err        error
}

type DisplayMode int

const (
    ModeLive DisplayMode = iota
    ModeCompact
    ModeTable
    ModeJSON
)

func New(mode DisplayMode) Model {
    return Model{
        mode: mode,
    }
}

func (m Model) Init() tea.Cmd {
    return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        }

    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height

    case tickMsg:
        // Refresh stats from aggregator
        return m, tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
            return tickMsg(t)
        })

    case statsMsg:
        m.stats = msg.stats
    }

    return m, nil
}

func (m Model) View() string {
    switch m.mode {
    case ModeLive:
        return m.renderLiveDashboard()
    case ModeCompact:
        return m.renderCompact()
    case ModeTable:
        return m.renderTable()
    case ModeJSON:
        return m.renderJSON()
    default:
        return ""
    }
}

func (m Model) renderLiveDashboard() string {
    if m.stats == nil {
        return "Loading..."
    }

    var b strings.Builder

    // Header
    header := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("39")).
        Render("Token Monitor")
    b.WriteString(header)
    b.WriteString(fmt.Sprintf(" - Session: %s\n", m.stats.SessionName))

    // Session info
    b.WriteString(fmt.Sprintf("Session ID: %s\n", truncateUUID(m.stats.SessionID)))
    b.WriteString(fmt.Sprintf("Project: %s\n", m.stats.ProjectPath))
    b.WriteString("\n")

    // Token usage
    b.WriteString("TOKEN USAGE\n")
    b.WriteString(m.renderTokenBar("Input", m.stats.InputTokens, m.stats.TotalTokens))
    b.WriteString(m.renderTokenBar("Output", m.stats.OutputTokens, m.stats.TotalTokens))
    b.WriteString(m.renderTokenBar("Cache Create", m.stats.CacheCreationTokens, m.stats.TotalTokens))
    b.WriteString(m.renderTokenBar("Cache Read", m.stats.CacheReadTokens, m.stats.TotalTokens))
    b.WriteString(fmt.Sprintf("Total: %s\n\n", formatNumber(m.stats.TotalTokens)))

    // Burn rate
    burnRateColor := getBurnRateColor(m.stats.BurnRate)
    b.WriteString(fmt.Sprintf("BURN RATE: %s tokens/min [%s]\n\n",
        formatNumber(int64(m.stats.BurnRate)),
        lipgloss.NewStyle().Foreground(burnRateColor).Render(getBurnRateLabel(m.stats.BurnRate))))

    // Current billing block
    if m.stats.CurrentBlock != nil {
        b.WriteString(m.renderBillingBlock(m.stats.CurrentBlock))
    }

    return b.String()
}

func (m Model) renderTokenBar(label string, value, total int64) string {
    percent := float64(value) / float64(total) * 100
    barWidth := 20
    filled := int(percent / 100 * float64(barWidth))

    bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

    return fmt.Sprintf("  %-12s %10s  %s  (%.0f%%)\n",
        label+":",
        formatNumber(value),
        bar,
        percent)
}
```

**Key Features**:
- **Bubbletea TUI**: Elm-inspired architecture
- **Responsive Layout**: Adapts to terminal size
- **Color Coding**: Visual indicators for burn rate, thresholds
- **Real-time Updates**: 1-second refresh rate
- **Multiple Modes**: Live, compact, table, JSON

---

## Configuration

### Configuration File Schema

**Location**: `~/.config/token-monitor/config.yaml`

```yaml
# Claude data directories
claude_config_dirs:
  - ~/.config/claude/projects/
  - ~/.claude/projects/

# Monitoring settings
monitoring:
  watch_interval: 1s          # How often to check for file changes
  update_frequency: 1s        # UI refresh rate
  session_retention: 720h     # 30 days

# Performance tuning
performance:
  worker_pool_size: 5         # Concurrent file processors
  cache_size: 100             # Max sessions in memory
  batch_window: 100ms         # Database write batching

# Display settings
display:
  default_mode: live          # live, compact, table, json
  color_enabled: true
  refresh_rate: 1s

# Storage
storage:
  db_path: ~/.config/token-monitor/sessions.db
  cache_dir: ~/.config/token-monitor/cache/

# Logging
logging:
  level: info                 # debug, info, warn, error
  output: stderr
```

### Environment Variables

- `CLAUDE_CONFIG_DIR`: Override Claude data directories (comma-separated)
- `TOKEN_MONITOR_CONFIG`: Custom config file path
- `TOKEN_MONITOR_DB`: Custom database path
- `TOKEN_MONITOR_LOG_LEVEL`: Override log level

### CLI Flags (highest precedence)

- `--config` - Custom config file
- `--log-level` - Logging level
- `--json` - JSON output mode
- `--no-color` - Disable colors

---

## Performance Targets

### Latency
- **File change detection**: <10ms (fsnotify)
- **Entry parsing**: <1ms per entry
- **Aggregation update**: <5ms per entry
- **UI refresh**: <50ms render time
- **End-to-end**: <100ms total

### Throughput
- **Parsing**: >10,000 entries/second
- **Database writes**: >1,000 updates/second (batched)
- **Concurrent sessions**: Support 100+ active sessions

### Resource Usage
- **Memory**: <50MB baseline, <200MB with 100 sessions
- **CPU**: <5% average, <20% during intensive operations
- **Disk I/O**: <10 IOPS sustained, <100 IOPS peak

---

## Error Handling Strategy

### Error Categories

1. **Transient Errors** (retry with backoff)
   - File locked
   - Database locked
   - Network filesystem delays

2. **Permanent Errors** (log and skip)
   - Malformed JSON
   - Invalid file paths
   - Permission denied

3. **Critical Errors** (fail fast)
   - Database corruption
   - Disk full
   - Out of memory

### Recovery Mechanisms

- **Exponential Backoff**: 100ms, 200ms, 400ms, ... max 5 seconds
- **Circuit Breaker**: Pause operations after 10 consecutive failures
- **Graceful Degradation**: Continue with reduced functionality
- **Automatic Restart**: Restart file watcher on errors

---

## Security Considerations

### Data Privacy
- **No Content Storage**: Only token counts and metadata
- **Read-Only Access**: Never modify Claude Code data
- **Local Storage**: All data stays on local machine
- **File Permissions**: 0600 for DB, 0700 for directories

### Input Validation
- **Path Sanitization**: Prevent path traversal
- **JSON Validation**: Strict schema enforcement
- **Size Limits**: Max 100MB per JSONL file
- **Resource Limits**: Hard memory limit (500MB)

### Dependency Management
- **Minimal Dependencies**: Only essential libraries
- **Security Scanning**: Regular vulnerability checks
- **Version Pinning**: Reproducible builds

---

## Testing Strategy

### Unit Tests
- **Coverage Target**: >80% overall, >95% for critical paths
- **Framework**: Go standard testing package
- **Mocking**: Interfaces for external dependencies
- **Table-Driven**: Comprehensive test cases

### Integration Tests
- **End-to-End**: Full monitoring flow
- **File System**: Simulate Claude Code writes
- **Database**: Persistence verification
- **Concurrency**: Race condition detection

### Performance Tests
- **Benchmarking**: Go benchmark framework
- **Load Testing**: 100 concurrent sessions
- **Memory Profiling**: 24-hour leak detection
- **I/O Profiling**: Throughput measurement

---

## Future Considerations

### Cost Analysis Integration
- Integrate LiteLLM pricing API
- Real-time cost calculations
- Budget tracking and alerts

### Web Dashboard
- REST API for stats
- WebSocket for live updates
- React frontend

### Team Features
- Multi-user aggregation
- Shared session tracking
- Cost allocation

### Export & Reporting
- CSV export
- PDF reports
- Prometheus metrics

---

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2025-11-17 | Use Go | Performance, concurrency, cross-platform |
| 2025-11-17 | BoltDB for storage | Embedded, zero-config, ACID |
| 2025-11-17 | fsnotify for watching | Cross-platform, battle-tested |
| 2025-11-17 | Bubbletea for TUI | Modern, Elm-inspired, beautiful |
| 2025-11-17 | Read-only data access | Data integrity, no side effects |
| 2025-11-17 | 5-hour billing blocks | Match Claude Code API limits |

---

## References

- Claude Code JSONL format: Analyzed from ccusage project
- Token tracking guide: `/Users/wm-it-22-00661/Work/github/ai/token-monitor/TOKEN_TRACKING_GUIDE.md`
- Architecture patterns: ccusage data-loader.ts, _live-monitor.ts
