// Package reader provides incremental file reading with position tracking.
//
// It reads files from the last known position and persists offsets to handle
// file rotation and application restarts.
//
// Example usage:
//
//	r, err := reader.New(reader.Config{
//	    PositionStore: sessionMgr,
//	    Parser:        parser.New(),
//	}, logger.Default())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer r.Close()
//
//	entries, err := r.Read(ctx, "/path/to/session.jsonl")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, entry := range entries {
//	    fmt.Printf("Tokens: %d\n", entry.Message.Usage.TotalTokens())
//	}
package reader

import (
	"context"
	"time"

	"github.com/yourusername/token-monitor/pkg/parser"
)

// PositionStore provides persistence for file read positions.
type PositionStore interface {
	// GetPosition retrieves the last read position for a file.
	//
	// Parameters:
	//   - path: Absolute file path
	//
	// Returns:
	//   - Last read offset in bytes
	//   - Error if retrieval fails
	//
	// Returns 0 if no position is stored (start from beginning).
	GetPosition(path string) (int64, error)

	// SetPosition stores the read position for a file.
	//
	// Parameters:
	//   - path: Absolute file path
	//   - offset: Current read offset in bytes
	//
	// Returns error if storage fails.
	SetPosition(path string, offset int64) error
}

// Reader provides incremental file reading.
type Reader interface {
	// Read reads new entries from a file since the last read position.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - path: Absolute path to JSONL file
	//
	// Returns:
	//   - Slice of new entries
	//   - Error if reading fails
	//
	// Automatically updates the stored position after successful read.
	Read(ctx context.Context, path string) ([]parser.UsageEntry, error)

	// ReadFrom reads entries from a specific offset.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - path: Absolute path to JSONL file
	//   - offset: Starting offset in bytes
	//
	// Returns:
	//   - Slice of entries
	//   - New offset after reading
	//   - Error if reading fails
	//
	// Does not update the stored position.
	ReadFrom(ctx context.Context, path string, offset int64) ([]parser.UsageEntry, int64, error)

	// Reset resets the read position for a file to the beginning.
	//
	// Parameters:
	//   - path: Absolute file path
	//
	// Returns error if reset fails.
	Reset(path string) error

	// Close closes the reader and releases resources.
	//
	// Returns error if cleanup fails.
	Close() error
}

// Config contains reader configuration.
type Config struct {
	// PositionStore persists file read positions.
	PositionStore PositionStore

	// Parser parses JSONL entries.
	Parser parser.Parser

	// MaxRetries is the maximum number of retry attempts for transient errors.
	// Default: 3.
	MaxRetries int

	// RetryDelay is the base delay between retry attempts.
	// Uses exponential backoff: delay * 2^attempt.
	// Default: 100ms.
	RetryDelay time.Duration

	// FileOpenTimeout is the maximum time to wait for file access.
	// Default: 5s.
	FileOpenTimeout time.Duration

	// MaxFileSize is the maximum file size to read (safety limit).
	// Default: 100MB.
	MaxFileSize int64
}
