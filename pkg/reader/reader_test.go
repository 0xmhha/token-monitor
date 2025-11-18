package reader

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yourusername/token-monitor/pkg/logger"
	"github.com/yourusername/token-monitor/pkg/parser"
)

func TestNew(t *testing.T) {
	store := NewMemoryPositionStore()
	p := parser.New()

	r, err := New(Config{
		PositionStore: store,
		Parser:        p,
	}, logger.Noop())

	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if r == nil {
		t.Error("New() returned nil reader")
	}

	if closeErr := r.Close(); closeErr != nil {
		t.Errorf("Close() error = %v", closeErr)
	}
}

func TestNewMissingStore(t *testing.T) {
	p := parser.New()

	_, err := New(Config{
		Parser: p,
	}, logger.Noop())

	if err == nil {
		t.Error("New() error = nil, want error for missing store")
	}
}

func TestNewMissingParser(t *testing.T) {
	store := NewMemoryPositionStore()

	_, err := New(Config{
		PositionStore: store,
	}, logger.Noop())

	if err == nil {
		t.Error("New() error = nil, want error for missing parser")
	}
}

func TestRead(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Create test file with JSONL content.
	content := `{"timestamp":"2024-01-01T00:00:00Z","sessionId":"a1b2c3d4-e5f6-7890-abcd-ef1234567890","message":{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":100,"output_tokens":50}}}
{"timestamp":"2024-01-01T00:01:00Z","sessionId":"a1b2c3d4-e5f6-7890-abcd-ef1234567890","message":{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":200,"output_tokens":100}}}
`
	if err := os.WriteFile(testFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	store := NewMemoryPositionStore()
	p := parser.New()

	r, err := New(Config{
		PositionStore: store,
		Parser:        p,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	ctx := context.Background()

	// First read should get all entries.
	entries, err := r.Read(ctx, testFile)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Read() returned %d entries, want 2", len(entries))
	}

	// Second read should get no new entries.
	entries, err = r.Read(ctx, testFile)
	if err != nil {
		t.Fatalf("Second Read() error = %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Second Read() returned %d entries, want 0", len(entries))
	}

	// Append new entry.
	newEntry := `{"timestamp":"2024-01-01T00:02:00Z","sessionId":"a1b2c3d4-e5f6-7890-abcd-ef1234567890","message":{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":150,"output_tokens":75}}}
`
	f, openErr := os.OpenFile(testFile, os.O_APPEND|os.O_WRONLY, 0600) // nolint:gosec // Test file with known path
	if openErr != nil {
		t.Fatalf("Failed to open file: %v", openErr)
	}
	if _, writeErr := f.WriteString(newEntry); writeErr != nil {
		if closeErr := f.Close(); closeErr != nil {
			t.Logf("Failed to close file: %v", closeErr)
		}
		t.Fatalf("Failed to append entry: %v", writeErr)
	}
	if closeErr := f.Close(); closeErr != nil {
		t.Logf("Failed to close file: %v", closeErr)
	}

	// Third read should get the new entry.
	entries, err = r.Read(ctx, testFile)
	if err != nil {
		t.Fatalf("Third Read() error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Third Read() returned %d entries, want 1", len(entries))
	}
}

func TestReadFrom(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	content := `{"timestamp":"2024-01-01T00:00:00Z","sessionId":"a1b2c3d4-e5f6-7890-abcd-ef1234567890","message":{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":100,"output_tokens":50}}}
{"timestamp":"2024-01-01T00:01:00Z","sessionId":"a1b2c3d4-e5f6-7890-abcd-ef1234567890","message":{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":200,"output_tokens":100}}}
`
	if err := os.WriteFile(testFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	store := NewMemoryPositionStore()
	p := parser.New()

	r, err := New(Config{
		PositionStore: store,
		Parser:        p,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	ctx := context.Background()

	// Read from beginning.
	entries, newOffset, err := r.ReadFrom(ctx, testFile, 0)
	if err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("ReadFrom() returned %d entries, want 2", len(entries))
	}

	if newOffset == 0 {
		t.Error("ReadFrom() newOffset = 0, want > 0")
	}

	// Verify position was not updated (ReadFrom doesn't update store).
	storedOffset, getErr := store.GetPosition(testFile)
	if getErr != nil {
		t.Fatalf("GetPosition() error = %v", getErr)
	}

	if storedOffset != 0 {
		t.Errorf("Stored offset = %d, want 0 (ReadFrom should not update)", storedOffset)
	}
}

func TestReadFromInvalidOffset(t *testing.T) {
	store := NewMemoryPositionStore()
	p := parser.New()

	r, err := New(Config{
		PositionStore: store,
		Parser:        p,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	ctx := context.Background()

	_, _, err = r.ReadFrom(ctx, "test.jsonl", -1)
	if err != ErrInvalidOffset {
		t.Errorf("ReadFrom() error = %v, want ErrInvalidOffset", err)
	}
}

func TestReadFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "nonexistent.jsonl")

	store := NewMemoryPositionStore()
	p := parser.New()

	r, err := New(Config{
		PositionStore: store,
		Parser:        p,
		MaxRetries:    0, // No retries for faster test.
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	ctx := context.Background()

	_, err = r.Read(ctx, nonExistent)
	if err == nil {
		t.Error("Read() error = nil, want error for non-existent file")
	}
}

func TestReadFileTruncated(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Create file.
	content := `{"timestamp":"2024-01-01T00:00:00Z","sessionId":"a1b2c3d4-e5f6-7890-abcd-ef1234567890","message":{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":100,"output_tokens":50}}}
`
	if err := os.WriteFile(testFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	store := NewMemoryPositionStore()
	p := parser.New()

	// Set position beyond file size (simulating truncation).
	if setErr := store.SetPosition(testFile, 10000); setErr != nil {
		t.Fatalf("SetPosition() error = %v", setErr)
	}

	r, err := New(Config{
		PositionStore: store,
		Parser:        p,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	ctx := context.Background()

	// Should reset to beginning and read all entries.
	entries, err := r.Read(ctx, testFile)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Read() returned %d entries, want 1", len(entries))
	}
}

func TestReset(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	content := `{"timestamp":"2024-01-01T00:00:00Z","sessionId":"a1b2c3d4-e5f6-7890-abcd-ef1234567890","message":{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":100,"output_tokens":50}}}
`
	if err := os.WriteFile(testFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	store := NewMemoryPositionStore()
	p := parser.New()

	r, err := New(Config{
		PositionStore: store,
		Parser:        p,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	ctx := context.Background()

	// Read file.
	entries, err := r.Read(ctx, testFile)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Read() returned %d entries, want 1", len(entries))
	}

	// Reset position.
	if resetErr := r.Reset(testFile); resetErr != nil {
		t.Fatalf("Reset() error = %v", resetErr)
	}

	// Read again should get the same entry.
	entries, err = r.Read(ctx, testFile)
	if err != nil {
		t.Fatalf("Second Read() error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Second Read() returned %d entries, want 1", len(entries))
	}
}

func TestReadClosed(t *testing.T) {
	store := NewMemoryPositionStore()
	p := parser.New()

	r, err := New(Config{
		PositionStore: store,
		Parser:        p,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if closeErr := r.Close(); closeErr != nil {
		t.Errorf("Close() error = %v", closeErr)
	}

	ctx := context.Background()

	_, err = r.Read(ctx, "test.jsonl")
	if err != ErrReaderClosed {
		t.Errorf("Read() error = %v, want ErrReaderClosed", err)
	}
}

func TestReadContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	content := `{"timestamp":"2024-01-01T00:00:00Z","sessionId":"a1b2c3d4-e5f6-7890-abcd-ef1234567890","message":{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":100,"output_tokens":50}}}
`
	if err := os.WriteFile(testFile, []byte(content), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	store := NewMemoryPositionStore()
	p := parser.New()

	r, err := New(Config{
		PositionStore: store,
		Parser:        p,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err = r.Read(ctx, testFile)
	if err != context.Canceled {
		t.Errorf("Read() error = %v, want context.Canceled", err)
	}
}

func TestCloseTwice(t *testing.T) {
	store := NewMemoryPositionStore()
	p := parser.New()

	r, err := New(Config{
		PositionStore: store,
		Parser:        p,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if closeErr := r.Close(); closeErr != nil {
		t.Errorf("First Close() error = %v", closeErr)
	}

	// Second close should not error.
	if closeErr := r.Close(); closeErr != nil {
		t.Errorf("Second Close() error = %v", closeErr)
	}
}

func TestMemoryPositionStore(t *testing.T) {
	store := NewMemoryPositionStore()

	// Get non-existent position.
	offset, err := store.GetPosition("/test/path")
	if err != nil {
		t.Fatalf("GetPosition() error = %v", err)
	}

	if offset != 0 {
		t.Errorf("GetPosition() = %d, want 0 for non-existent path", offset)
	}

	// Set position.
	if setErr := store.SetPosition("/test/path", 12345); setErr != nil {
		t.Fatalf("SetPosition() error = %v", setErr)
	}

	// Get position.
	offset, err = store.GetPosition("/test/path")
	if err != nil {
		t.Fatalf("GetPosition() error = %v", err)
	}

	if offset != 12345 {
		t.Errorf("GetPosition() = %d, want 12345", offset)
	}

	// Update position.
	if setErr := store.SetPosition("/test/path", 67890); setErr != nil {
		t.Fatalf("SetPosition() error = %v", setErr)
	}

	// Get updated position.
	offset, err = store.GetPosition("/test/path")
	if err != nil {
		t.Fatalf("GetPosition() error = %v", err)
	}

	if offset != 67890 {
		t.Errorf("GetPosition() = %d, want 67890", offset)
	}
}

func TestReadEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.jsonl")

	// Create empty file.
	if err := os.WriteFile(testFile, []byte(""), 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	store := NewMemoryPositionStore()
	p := parser.New()

	r, err := New(Config{
		PositionStore: store,
		Parser:        p,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	ctx := context.Background()

	entries, err := r.Read(ctx, testFile)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Read() returned %d entries, want 0 for empty file", len(entries))
	}
}

func TestReadWithRetry(t *testing.T) {
	store := NewMemoryPositionStore()
	p := parser.New()

	r, err := New(Config{
		PositionStore: store,
		Parser:        p,
		MaxRetries:    2,
		RetryDelay:    10 * time.Millisecond,
	}, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	ctx := context.Background()

	// File doesn't exist, should retry.
	start := time.Now()
	_, err = r.Read(ctx, testFile)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Read() error = nil, want error for non-existent file")
	}

	// Should have retried (total attempts = 3: initial + 2 retries).
	// Minimum time: 2 retries * 10ms = 20ms.
	if elapsed < 20*time.Millisecond {
		t.Errorf("Read() took %v, expected at least 20ms for retries", elapsed)
	}

	t.Logf("Read with retries took %v for non-existent file", elapsed)
}
