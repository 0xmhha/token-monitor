// Package watcher provides real-time file system monitoring.
//
// It uses fsnotify to watch for changes to Claude Code session files
// and provides event debouncing to handle rapid file updates.
//
// Example usage:
//
//	w, err := watcher.New(watcher.Config{
//	    DebounceInterval: 100 * time.Millisecond,
//	}, logger.Default())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer w.Close()
//
//	ctx := context.Background()
//	paths := []string{"~/.config/claude/projects"}
//
//	if err := w.Start(ctx, paths); err != nil {
//	    log.Fatal(err)
//	}
//
//	for event := range w.Events() {
//	    fmt.Printf("File %s: %s\n", event.Path, event.Op)
//	}
package watcher

import (
	"context"
	"time"
)

// Op describes a file operation type.
type Op uint32

// File operation types.
const (
	OpCreate Op = 1 << iota // File created
	OpWrite                 // File modified
	OpRemove                // File deleted
	OpRename                // File renamed/moved
	OpChmod                 // File permissions changed
)

// String returns a human-readable operation name.
func (op Op) String() string {
	switch op {
	case OpCreate:
		return "CREATE"
	case OpWrite:
		return "WRITE"
	case OpRemove:
		return "REMOVE"
	case OpRename:
		return "RENAME"
	case OpChmod:
		return "CHMOD"
	default:
		return "UNKNOWN"
	}
}

// Event represents a file system event.
type Event struct {
	// Path is the absolute path to the file that triggered the event.
	Path string

	// Op is the operation that triggered the event.
	Op Op

	// Timestamp is when the event occurred.
	Timestamp time.Time
}

// Watcher provides file system monitoring.
type Watcher interface {
	// Start begins watching the specified paths.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - paths: Directories to watch
	//
	// Returns error if watching cannot be started.
	//
	// Note: This method blocks until context is cancelled or an error occurs.
	Start(ctx context.Context, paths []string) error

	// Stop gracefully shuts down the watcher.
	//
	// Returns error if shutdown fails.
	Stop() error

	// Events returns the channel for receiving file system events.
	//
	// Events are debounced based on the configured interval.
	// The channel is closed when the watcher stops.
	Events() <-chan Event

	// Errors returns the channel for receiving watcher errors.
	//
	// Non-fatal errors are sent to this channel.
	// The channel is closed when the watcher stops.
	Errors() <-chan error

	// Close closes the watcher and releases resources.
	//
	// Returns error if resources cannot be released cleanly.
	Close() error
}

// Config contains watcher configuration.
type Config struct {
	// DebounceInterval is the time to wait before emitting an event.
	// Multiple events for the same file within this interval are coalesced.
	// Default: 100ms.
	DebounceInterval time.Duration

	// MaxRetries is the maximum number of retry attempts for watcher errors.
	// Default: 3.
	MaxRetries int

	// RetryDelay is the delay between retry attempts.
	// Uses exponential backoff: delay * 2^attempt.
	// Default: 1s.
	RetryDelay time.Duration

	// CircuitBreakerThreshold is the number of consecutive failures
	// before the circuit breaker opens (stops retrying).
	// Default: 5.
	CircuitBreakerThreshold int
}
