package monitor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/token-monitor/pkg/discovery"
	"github.com/yourusername/token-monitor/pkg/logger"
	"github.com/yourusername/token-monitor/pkg/parser"
	"github.com/yourusername/token-monitor/pkg/watcher"
)

// mockWatcher implements the watcher.Watcher interface for testing.
type mockWatcher struct {
	mu        sync.Mutex
	started   bool
	stopped   bool
	closed    bool
	paths     []string
	events    chan watcher.Event
	errors    chan error
	startErr  error
	stopErr   error
	closeErr  error
}

func newMockWatcher() *mockWatcher {
	return &mockWatcher{
		events: make(chan watcher.Event, 10),
		errors: make(chan error, 10),
	}
}

func (m *mockWatcher) Start(ctx context.Context, paths []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return m.startErr
	}
	m.started = true
	m.paths = paths
	return nil
}

func (m *mockWatcher) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopErr != nil {
		return m.stopErr
	}
	m.stopped = true
	return nil
}

func (m *mockWatcher) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closeErr != nil {
		return m.closeErr
	}
	m.closed = true
	close(m.events)
	close(m.errors)
	return nil
}

func (m *mockWatcher) Events() <-chan watcher.Event {
	return m.events
}

func (m *mockWatcher) Errors() <-chan error {
	return m.errors
}

func (m *mockWatcher) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started && !m.stopped
}

func (m *mockWatcher) Started() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

func (m *mockWatcher) Stopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
}

func (m *mockWatcher) Paths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.paths...)
}

// mockReader implements the reader.Reader interface for testing.
type mockReader struct {
	mu       sync.Mutex
	entries  map[string][]parser.UsageEntry
	readErr  error
	resetErr error
	closed   bool
}

func newMockReader() *mockReader {
	return &mockReader{
		entries: make(map[string][]parser.UsageEntry),
	}
}

func (m *mockReader) Read(ctx context.Context, path string) ([]parser.UsageEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readErr != nil {
		return nil, m.readErr
	}
	entries := m.entries[path]
	// Clear entries after reading (simulating incremental read)
	m.entries[path] = nil
	return entries, nil
}

func (m *mockReader) ReadFrom(ctx context.Context, path string, offset int64) ([]parser.UsageEntry, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readErr != nil {
		return nil, 0, m.readErr
	}
	entries := m.entries[path]
	m.entries[path] = nil
	return entries, offset + 100, nil
}

func (m *mockReader) Reset(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resetErr
}

func (m *mockReader) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockReader) SetEntries(path string, entries []parser.UsageEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[path] = entries
}

// mockDiscovery implements the discovery.Discoverer interface for testing.
type mockDiscovery struct {
	sessions    []discovery.SessionFile
	discoverErr error
}

func newMockDiscovery(sessions []discovery.SessionFile) *mockDiscovery {
	return &mockDiscovery{
		sessions: sessions,
	}
}

func (m *mockDiscovery) Discover() ([]discovery.SessionFile, error) {
	if m.discoverErr != nil {
		return nil, m.discoverErr
	}
	return m.sessions, nil
}

func (m *mockDiscovery) DiscoverProject(projectPath string) ([]discovery.SessionFile, error) {
	if m.discoverErr != nil {
		return nil, m.discoverErr
	}
	// Filter sessions by project path
	var filtered []discovery.SessionFile
	for _, s := range m.sessions {
		if s.ProjectPath == projectPath {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

// Helper to create test entries.
func createTestEntry(sessionID string, tokens int) parser.UsageEntry {
	return parser.UsageEntry{
		SessionID: sessionID,
		Timestamp: time.Now(),
		Message: parser.Message{
			Model: "claude-3-sonnet",
			Usage: parser.Usage{
				InputTokens:  tokens / 2,
				OutputTokens: tokens / 2,
			},
		},
	}
}

func TestNew(t *testing.T) {
	log := logger.New(logger.Config{Level: "error"})
	w := newMockWatcher()
	r := newMockReader()
	d := newMockDiscovery(nil)

	t.Run("creates monitor with default config", func(t *testing.T) {
		mon, err := New(Config{}, w, r, d, log)
		require.NoError(t, err)
		assert.NotNil(t, mon)
	})

	t.Run("sets default refresh interval", func(t *testing.T) {
		mon, err := New(Config{}, w, r, d, log)
		require.NoError(t, err)
		lm := mon.(*liveMonitor)
		assert.Equal(t, time.Second, lm.config.RefreshInterval)
	})

	t.Run("uses custom refresh interval", func(t *testing.T) {
		mon, err := New(Config{RefreshInterval: 5 * time.Second}, w, r, d, log)
		require.NoError(t, err)
		lm := mon.(*liveMonitor)
		assert.Equal(t, 5*time.Second, lm.config.RefreshInterval)
	})

	t.Run("accepts session filter", func(t *testing.T) {
		sessionIDs := []string{"session-1", "session-2"}
		mon, err := New(Config{SessionIDs: sessionIDs}, w, r, d, log)
		require.NoError(t, err)
		lm := mon.(*liveMonitor)
		assert.Equal(t, sessionIDs, lm.config.SessionIDs)
	})
}

func TestStart(t *testing.T) {
	log := logger.New(logger.Config{Level: "error"})

	t.Run("starts successfully with sessions", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		sessions := []discovery.SessionFile{
			{SessionID: "session-1", FilePath: "/path/to/session1.jsonl"},
		}
		d := newMockDiscovery(sessions)

		// Set up initial entries
		r.SetEntries("/path/to/session1.jsonl", []parser.UsageEntry{
			createTestEntry("session-1", 100),
		})

		mon, err := New(Config{RefreshInterval: 100 * time.Millisecond}, w, r, d, log)
		require.NoError(t, err)

		// Start in goroutine
		errChan := make(chan error, 1)
		go func() {
			errChan <- mon.Start()
		}()

		// Wait a bit for startup
		time.Sleep(50 * time.Millisecond)

		// Verify watcher started
		assert.True(t, w.Started())
		assert.Contains(t, w.Paths(), "/path/to/session1.jsonl")

		// Stop monitor
		err = mon.Stop()
		require.NoError(t, err)
	})

	t.Run("returns error when no sessions found", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		d := newMockDiscovery([]discovery.SessionFile{})

		mon, err := New(Config{}, w, r, d, log)
		require.NoError(t, err)

		err = mon.Start()
		assert.Equal(t, ErrNoSessions, err)
	})

	t.Run("returns error when discovery fails", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		d := newMockDiscovery(nil)
		d.discoverErr = assert.AnError

		mon, err := New(Config{}, w, r, d, log)
		require.NoError(t, err)

		err = mon.Start()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to discover sessions")
	})

	t.Run("returns error when already running", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		sessions := []discovery.SessionFile{
			{SessionID: "session-1", FilePath: "/path/to/session1.jsonl"},
		}
		d := newMockDiscovery(sessions)

		mon, err := New(Config{RefreshInterval: 100 * time.Millisecond}, w, r, d, log)
		require.NoError(t, err)

		// Start first time
		go func() {
			_ = mon.Start() // Error handled by monitor
		}()
		time.Sleep(50 * time.Millisecond)

		// Try to start again
		err = mon.Start()
		assert.Equal(t, ErrMonitorRunning, err)

		// Cleanup
		_ = mon.Stop() // Ignore error in test cleanup
	})

	t.Run("returns error when closed", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		sessions := []discovery.SessionFile{
			{SessionID: "session-1", FilePath: "/path/to/session1.jsonl"},
		}
		d := newMockDiscovery(sessions)

		mon, err := New(Config{}, w, r, d, log)
		require.NoError(t, err)

		// Close monitor
		lm := mon.(*liveMonitor)
		_ = lm.Close() // Ignore error in test

		// Try to start
		err = mon.Start()
		assert.Equal(t, ErrMonitorClosed, err)
	})

	t.Run("filters sessions by ID", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		sessions := []discovery.SessionFile{
			{SessionID: "session-1", FilePath: "/path/to/session1.jsonl"},
			{SessionID: "session-2", FilePath: "/path/to/session2.jsonl"},
			{SessionID: "session-3", FilePath: "/path/to/session3.jsonl"},
		}
		d := newMockDiscovery(sessions)

		mon, err := New(Config{
			SessionIDs:      []string{"session-1", "session-3"},
			RefreshInterval: 100 * time.Millisecond,
		}, w, r, d, log)
		require.NoError(t, err)

		go func() {
			_ = mon.Start() // Error handled by monitor
		}()
		time.Sleep(50 * time.Millisecond)

		// Should only watch filtered sessions
		paths := w.Paths()
		assert.Len(t, paths, 2)
		assert.Contains(t, paths, "/path/to/session1.jsonl")
		assert.Contains(t, paths, "/path/to/session3.jsonl")
		assert.NotContains(t, paths, "/path/to/session2.jsonl")

		_ = mon.Stop() // Ignore error in test cleanup
	})
}

func TestStop(t *testing.T) {
	log := logger.New(logger.Config{Level: "error"})

	t.Run("stops running monitor", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		sessions := []discovery.SessionFile{
			{SessionID: "session-1", FilePath: "/path/to/session1.jsonl"},
		}
		d := newMockDiscovery(sessions)

		mon, err := New(Config{RefreshInterval: 100 * time.Millisecond}, w, r, d, log)
		require.NoError(t, err)

		go func() {
			_ = mon.Start() // Error handled by monitor
		}()
		time.Sleep(50 * time.Millisecond)

		err = mon.Stop()
		require.NoError(t, err)
		assert.True(t, w.Stopped())
	})

	t.Run("returns error when not running", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		d := newMockDiscovery(nil)

		mon, err := New(Config{}, w, r, d, log)
		require.NoError(t, err)

		err = mon.Stop()
		assert.Equal(t, ErrMonitorNotRunning, err)
	})

	t.Run("returns error when closed", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		d := newMockDiscovery(nil)

		mon, err := New(Config{}, w, r, d, log)
		require.NoError(t, err)

		lm := mon.(*liveMonitor)
		_ = lm.Close() // Ignore error in test

		err = mon.Stop()
		assert.Equal(t, ErrMonitorClosed, err)
	})
}

func TestStats(t *testing.T) {
	log := logger.New(logger.Config{Level: "error"})

	t.Run("returns aggregated stats", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		sessions := []discovery.SessionFile{
			{SessionID: "session-1", FilePath: "/path/to/session1.jsonl"},
		}
		d := newMockDiscovery(sessions)

		// Set up entries
		r.SetEntries("/path/to/session1.jsonl", []parser.UsageEntry{
			createTestEntry("session-1", 100),
			createTestEntry("session-1", 200),
		})

		mon, err := New(Config{RefreshInterval: 100 * time.Millisecond}, w, r, d, log)
		require.NoError(t, err)

		go func() {
			_ = mon.Start() // Error handled by monitor
		}()
		time.Sleep(50 * time.Millisecond)

		stats := mon.Stats()
		assert.Equal(t, 2, stats.Count)
		assert.Equal(t, 300, stats.TotalTokens)

		_ = mon.Stop() // Ignore error in test cleanup
	})
}

func TestUpdates(t *testing.T) {
	log := logger.New(logger.Config{Level: "error"})

	t.Run("receives updates on channel", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		sessions := []discovery.SessionFile{
			{SessionID: "session-1", FilePath: "/path/to/session1.jsonl"},
		}
		d := newMockDiscovery(sessions)

		// Set up initial entries
		r.SetEntries("/path/to/session1.jsonl", []parser.UsageEntry{
			createTestEntry("session-1", 100),
		})

		mon, err := New(Config{RefreshInterval: 50 * time.Millisecond}, w, r, d, log)
		require.NoError(t, err)

		go func() {
			_ = mon.Start() // Error handled by monitor
		}()

		// Get updates channel
		lm := mon.(*liveMonitor)
		updates := lm.Updates()

		// Should receive initial update
		select {
		case update := <-updates:
			assert.Equal(t, 1, update.Stats.Count)
			assert.Equal(t, 100, update.Stats.TotalTokens)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("did not receive initial update")
		}

		_ = mon.Stop() // Ignore error in test cleanup
	})

	t.Run("includes burn rate and billing block", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		sessions := []discovery.SessionFile{
			{SessionID: "session-1", FilePath: "/path/to/session1.jsonl"},
		}
		d := newMockDiscovery(sessions)

		// Set up entries
		r.SetEntries("/path/to/session1.jsonl", []parser.UsageEntry{
			createTestEntry("session-1", 1000),
		})

		mon, err := New(Config{RefreshInterval: 50 * time.Millisecond}, w, r, d, log)
		require.NoError(t, err)

		go func() {
			_ = mon.Start() // Error handled by monitor
		}()

		lm := mon.(*liveMonitor)
		updates := lm.Updates()

		select {
		case update := <-updates:
			// Burn rate should be calculated
			assert.GreaterOrEqual(t, update.BurnRate.EntryCount, 0)
			// Current block should have valid times
			assert.False(t, update.CurrentBlock.StartTime.IsZero())
		case <-time.After(200 * time.Millisecond):
			t.Fatal("did not receive update")
		}

		_ = mon.Stop() // Ignore error in test cleanup
	})

	t.Run("tracks cumulative changes", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		sessions := []discovery.SessionFile{
			{SessionID: "session-1", FilePath: "/path/to/session1.jsonl"},
		}
		d := newMockDiscovery(sessions)

		// Initial entries
		r.SetEntries("/path/to/session1.jsonl", []parser.UsageEntry{
			createTestEntry("session-1", 100),
		})

		mon, err := New(Config{RefreshInterval: 30 * time.Millisecond}, w, r, d, log)
		require.NoError(t, err)

		go func() {
			_ = mon.Start() // Error handled by monitor
		}()

		lm := mon.(*liveMonitor)
		updates := lm.Updates()

		// Get first update
		var firstUpdate Update
		select {
		case firstUpdate = <-updates:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("did not receive first update")
		}

		// Add more entries
		r.SetEntries("/path/to/session1.jsonl", []parser.UsageEntry{
			createTestEntry("session-1", 200),
		})

		// Wait for next update
		time.Sleep(50 * time.Millisecond)

		// Get second update
		var secondUpdate Update
		select {
		case secondUpdate = <-updates:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("did not receive second update")
		}

		// Cumulative should show total change since start
		assert.GreaterOrEqual(t, secondUpdate.Stats.TotalTokens, firstUpdate.Stats.TotalTokens)

		_ = mon.Stop() // Ignore error in test cleanup
	})
}

func TestConcurrency(t *testing.T) {
	log := logger.New(logger.Config{Level: "error"})

	t.Run("handles concurrent operations", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		sessions := []discovery.SessionFile{
			{SessionID: "session-1", FilePath: "/path/to/session1.jsonl"},
		}
		d := newMockDiscovery(sessions)

		mon, err := New(Config{RefreshInterval: 10 * time.Millisecond}, w, r, d, log)
		require.NoError(t, err)

		go func() {
			_ = mon.Start() // Error handled by monitor
		}()
		time.Sleep(20 * time.Millisecond)

		// Concurrent stats calls
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = mon.Stats()
			}()
		}
		wg.Wait()

		_ = mon.Stop() // Ignore error in test cleanup
	})
}

func TestClose(t *testing.T) {
	log := logger.New(logger.Config{Level: "error"})

	t.Run("closes monitor", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		d := newMockDiscovery(nil)

		mon, err := New(Config{}, w, r, d, log)
		require.NoError(t, err)

		lm := mon.(*liveMonitor)
		err = lm.Close()
		require.NoError(t, err)
		assert.True(t, lm.closed)
	})

	t.Run("idempotent close", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		d := newMockDiscovery(nil)

		mon, err := New(Config{}, w, r, d, log)
		require.NoError(t, err)

		lm := mon.(*liveMonitor)
		err = lm.Close()
		require.NoError(t, err)

		err = lm.Close()
		require.NoError(t, err)
	})

	t.Run("stops running monitor on close", func(t *testing.T) {
		w := newMockWatcher()
		r := newMockReader()
		sessions := []discovery.SessionFile{
			{SessionID: "session-1", FilePath: "/path/to/session1.jsonl"},
		}
		d := newMockDiscovery(sessions)

		mon, err := New(Config{RefreshInterval: 100 * time.Millisecond}, w, r, d, log)
		require.NoError(t, err)

		go func() {
			_ = mon.Start() // Error handled by monitor
		}()
		time.Sleep(50 * time.Millisecond)

		lm := mon.(*liveMonitor)
		err = lm.Close()
		require.NoError(t, err)
		assert.False(t, lm.running)
	})
}
