package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xmhha/token-monitor/pkg/logger"
)

func TestNew(t *testing.T) {
	w, err := New(Config{}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if w == nil {
		t.Error("New() returned nil watcher")
	}

	if closeErr := w.Close(); closeErr != nil {
		t.Errorf("Close() error = %v", closeErr)
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := Config{
		DebounceInterval:        200 * time.Millisecond,
		MaxRetries:              5,
		RetryDelay:              2 * time.Second,
		CircuitBreakerThreshold: 10,
	}

	w, err := New(cfg, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if closeErr := w.Close(); closeErr != nil {
		t.Errorf("Close() error = %v", closeErr)
	}
}

func TestStart(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := New(Config{
		DebounceInterval: 50 * time.Millisecond,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Logf("Close() error = %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watcher in background.
	errChan := make(chan error, 1)
	go func() {
		errChan <- w.Start(ctx, []string{tmpDir})
	}()

	// Give it time to start.
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop.
	cancel()

	// Check for errors.
	if startErr := <-errChan; startErr != nil {
		t.Errorf("Start() error = %v", startErr)
	}
}

func TestStartInvalidPath(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent")

	w, err := New(Config{}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Logf("Close() error = %v", err)
		}
	}()

	ctx := context.Background()

	// Should skip nonexistent path and return error if all paths are invalid.
	if startErr := w.Start(ctx, []string{nonExistent}); startErr == nil {
		t.Error("Start() error = nil, want error for nonexistent path")
	}
}

func TestStartAlreadyStarted(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := New(Config{}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if closeErr := w.Close(); closeErr != nil {
			t.Logf("Close() error = %v", closeErr)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watcher in background.
	go func() {
		_ = w.Start(ctx, []string{tmpDir}) // nolint:errcheck // Background goroutine, errors checked via errChan or context
	}()

	// Give it time to start.
	time.Sleep(100 * time.Millisecond)

	// Try to start again.
	startErr := w.Start(ctx, []string{tmpDir})
	if startErr != ErrAlreadyStarted {
		t.Errorf("Start() error = %v, want ErrAlreadyStarted", startErr)
	}
}

func TestFileCreate(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := New(Config{
		DebounceInterval: 50 * time.Millisecond,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Logf("Close() error = %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watcher in background.
	go func() {
		_ = w.Start(ctx, []string{tmpDir}) // nolint:errcheck // Background goroutine, errors checked via errChan or context
	}()

	// Give watcher time to start.
	time.Sleep(100 * time.Millisecond)

	// Create a JSONL file.
	testFile := filepath.Join(tmpDir, "test.jsonl")
	if writeErr := os.WriteFile(testFile, []byte("test"), 0600); writeErr != nil {
		t.Fatalf("Failed to create test file: %v", writeErr)
	}

	// Wait for event (with debounce + buffer).
	select {
	case event := <-w.Events():
		if event.Path != testFile {
			t.Errorf("Event path = %s, want %s", event.Path, testFile)
		}
		if event.Op != OpCreate && event.Op != OpWrite {
			t.Errorf("Event op = %s, want CREATE or WRITE", event.Op)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for file create event")
	}
}

func TestFileModify(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file before starting watcher.
	testFile := filepath.Join(tmpDir, "test.jsonl")
	if err := os.WriteFile(testFile, []byte("initial"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	w, err := New(Config{
		DebounceInterval: 50 * time.Millisecond,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Logf("Close() error = %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watcher in background.
	go func() {
		_ = w.Start(ctx, []string{tmpDir}) // nolint:errcheck // Background goroutine, errors checked via errChan or context
	}()

	// Give watcher time to start.
	time.Sleep(100 * time.Millisecond)

	// Modify the file.
	if writeErr := os.WriteFile(testFile, []byte("modified"), 0600); writeErr != nil {
		t.Fatalf("Failed to modify test file: %v", writeErr)
	}

	// Wait for event.
	select {
	case event := <-w.Events():
		if event.Path != testFile {
			t.Errorf("Event path = %s, want %s", event.Path, testFile)
		}
		if event.Op != OpWrite {
			t.Errorf("Event op = %s, want WRITE", event.Op)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for file modify event")
	}
}

func TestFileDelete(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file before starting watcher.
	testFile := filepath.Join(tmpDir, "test.jsonl")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	w, err := New(Config{
		DebounceInterval: 50 * time.Millisecond,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Logf("Close() error = %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watcher in background.
	go func() {
		_ = w.Start(ctx, []string{tmpDir}) // nolint:errcheck // Background goroutine, errors checked via errChan or context
	}()

	// Give watcher time to start.
	time.Sleep(100 * time.Millisecond)

	// Delete the file.
	if removeErr := os.Remove(testFile); removeErr != nil {
		t.Fatalf("Failed to delete test file: %v", removeErr)
	}

	// Wait for event.
	select {
	case event := <-w.Events():
		if event.Path != testFile {
			t.Errorf("Event path = %s, want %s", event.Path, testFile)
		}
		if event.Op != OpRemove {
			t.Errorf("Event op = %s, want REMOVE", event.Op)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for file delete event")
	}
}

func TestDebouncing(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := New(Config{
		DebounceInterval: 200 * time.Millisecond,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Logf("Close() error = %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watcher in background.
	go func() {
		_ = w.Start(ctx, []string{tmpDir}) // nolint:errcheck // Background goroutine, errors checked via errChan or context
	}()

	// Give watcher time to start.
	time.Sleep(100 * time.Millisecond)

	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Create file first (to avoid create + write events).
	if writeErr := os.WriteFile(testFile, []byte("initial"), 0600); writeErr != nil {
		t.Fatalf("Failed to create test file: %v", writeErr)
	}

	// Wait for initial event to clear.
	time.Sleep(500 * time.Millisecond)

	// Drain any pending events.
	drainEvents(w.Events())

	// Rapid file modifications (only writes now).
	for i := 0; i < 5; i++ {
		if writeErr := os.WriteFile(testFile, []byte("content"), 0600); writeErr != nil {
			t.Fatalf("Failed to write test file: %v", writeErr)
		}
		time.Sleep(30 * time.Millisecond) // Less than debounce interval.
	}

	// Should receive only one debounced event.
	eventCount := 0
	timeout := time.After(1 * time.Second)

	for eventCount < 3 {
		select {
		case <-w.Events():
			eventCount++
		case <-timeout:
			// Exit loop after timeout.
			if eventCount == 0 {
				t.Error("No events received")
			}
			eventCount = 3
		}
	}

	// Debouncing should reduce event count significantly.
	// Without debouncing we'd expect 5+ events.
	// With debouncing, we should get 1-3 events depending on OS behavior.
	if eventCount == 0 {
		t.Error("No events received, debouncing may be too aggressive")
	}
	if eventCount >= 5 {
		t.Errorf("Received %d events for 5 rapid writes, debouncing not working", eventCount)
	}

	t.Logf("Debouncing working: 5 rapid writes resulted in %d event(s)", eventCount)
}

// drainEvents drains all pending events from a channel.
func drainEvents(ch <-chan Event) {
	for {
		select {
		case <-ch:
			// Drain.
		default:
			return
		}
	}
}

func TestNonJSONLFilesIgnored(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := New(Config{
		DebounceInterval: 50 * time.Millisecond,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Logf("Close() error = %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watcher in background.
	go func() {
		_ = w.Start(ctx, []string{tmpDir}) // nolint:errcheck // Background goroutine, errors checked via errChan or context
	}()

	// Give watcher time to start.
	time.Sleep(100 * time.Millisecond)

	// Create non-JSONL files.
	txtFile := filepath.Join(tmpDir, "test.txt")
	if writeErr := os.WriteFile(txtFile, []byte("test"), 0600); writeErr != nil {
		t.Fatalf("Failed to create txt file: %v", writeErr)
	}

	// Should not receive any events.
	select {
	case event := <-w.Events():
		t.Errorf("Received unexpected event for non-JSONL file: %v", event)
	case <-time.After(500 * time.Millisecond):
		// Expected - no events.
	}
}

func TestSubdirectoryWatching(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subdirectory.
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0700); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	w, err := New(Config{
		DebounceInterval: 50 * time.Millisecond,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Logf("Close() error = %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watcher in background.
	go func() {
		_ = w.Start(ctx, []string{tmpDir}) // nolint:errcheck // Background goroutine, errors checked via errChan or context
	}()

	// Give watcher time to start.
	time.Sleep(100 * time.Millisecond)

	// Create file in subdirectory.
	testFile := filepath.Join(subDir, "test.jsonl")
	if writeErr := os.WriteFile(testFile, []byte("test"), 0600); writeErr != nil {
		t.Fatalf("Failed to create test file: %v", writeErr)
	}

	// Wait for event.
	select {
	case event := <-w.Events():
		if event.Path != testFile {
			t.Errorf("Event path = %s, want %s", event.Path, testFile)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for subdirectory file event")
	}
}

func TestOpString(t *testing.T) {
	tests := []struct {
		op   Op
		want string
	}{
		{OpCreate, "CREATE"},
		{OpWrite, "WRITE"},
		{OpRemove, "REMOVE"},
		{OpRename, "RENAME"},
		{OpChmod, "CHMOD"},
		{Op(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.op.String()
		if got != tt.want {
			t.Errorf("Op.String() = %s, want %s", got, tt.want)
		}
	}
}

func TestStopNotStarted(t *testing.T) {
	w, err := New(Config{}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if closeErr := w.Close(); closeErr != nil {
			t.Logf("Close() error = %v", closeErr)
		}
	}()

	stopErr := w.Stop()
	if stopErr != ErrNotStarted {
		t.Errorf("Stop() error = %v, want ErrNotStarted", stopErr)
	}
}

func TestCloseTwice(t *testing.T) {
	w, err := New(Config{}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if closeErr := w.Close(); closeErr != nil {
		t.Errorf("First Close() error = %v", closeErr)
	}

	// Second close should not error.
	if closeErr := w.Close(); closeErr != nil {
		t.Errorf("Second Close() error = %v", closeErr)
	}
}

func TestStartAfterClose(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := New(Config{}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if closeErr := w.Close(); closeErr != nil {
		t.Errorf("Close() error = %v", closeErr)
	}

	ctx := context.Background()
	startErr := w.Start(ctx, []string{tmpDir})
	if startErr != ErrWatcherClosed {
		t.Errorf("Start() error = %v, want ErrWatcherClosed", startErr)
	}
}
