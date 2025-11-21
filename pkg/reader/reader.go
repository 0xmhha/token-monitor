package reader

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/0xmhha/token-monitor/pkg/logger"
	"github.com/0xmhha/token-monitor/pkg/parser"
)

// reader implements the Reader interface.
type reader struct {
	store  PositionStore
	parser parser.Parser
	logger logger.Logger
	config Config

	mu     sync.RWMutex
	closed bool
}

// New creates a new incremental file reader.
//
// Parameters:
//   - cfg: Reader configuration
//   - log: Logger instance
//
// Returns:
//   - Configured Reader
//   - Error if configuration is invalid
func New(cfg Config, log logger.Logger) (Reader, error) {
	if cfg.PositionStore == nil {
		return nil, fmt.Errorf("position store is required")
	}

	if cfg.Parser == nil {
		return nil, fmt.Errorf("parser is required")
	}

	// Set defaults.
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 100 * time.Millisecond
	}
	if cfg.FileOpenTimeout == 0 {
		cfg.FileOpenTimeout = 5 * time.Second
	}
	if cfg.MaxFileSize == 0 {
		cfg.MaxFileSize = 100 * 1024 * 1024 // 100MB
	}

	log.Info("incremental reader created",
		"max_retries", cfg.MaxRetries,
		"retry_delay", cfg.RetryDelay,
		"max_file_size", cfg.MaxFileSize)

	return &reader{
		store:  cfg.PositionStore,
		parser: cfg.Parser,
		logger: log,
		config: cfg,
	}, nil
}

// Read implements Reader.Read.
func (r *reader) Read(ctx context.Context, path string) ([]parser.UsageEntry, error) {
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return nil, ErrReaderClosed
	}
	r.mu.RUnlock()

	// Get last read position.
	offset, err := r.store.GetPosition(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get position: %w", err)
	}

	r.logger.Debug("reading file",
		"path", path,
		"offset", offset)

	// Read entries.
	entries, newOffset, err := r.readWithRetry(ctx, path, offset)
	if err != nil {
		return nil, err
	}

	// Update position.
	if err := r.store.SetPosition(path, newOffset); err != nil {
		r.logger.Error("failed to update position",
			"path", path,
			"offset", newOffset,
			"error", err)
		// Don't fail the read, just log the error.
	}

	r.logger.Debug("read complete",
		"path", path,
		"entries", len(entries),
		"new_offset", newOffset)

	return entries, nil
}

// ReadFrom implements Reader.ReadFrom.
func (r *reader) ReadFrom(ctx context.Context, path string, offset int64) ([]parser.UsageEntry, int64, error) {
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return nil, 0, ErrReaderClosed
	}
	r.mu.RUnlock()

	if offset < 0 {
		return nil, 0, ErrInvalidOffset
	}

	return r.readWithRetry(ctx, path, offset)
}

// Reset implements Reader.Reset.
func (r *reader) Reset(path string) error {
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return ErrReaderClosed
	}
	r.mu.RUnlock()

	if err := r.store.SetPosition(path, 0); err != nil {
		return fmt.Errorf("failed to reset position: %w", err)
	}

	r.logger.Info("position reset", "path", path)
	return nil
}

// Close implements Reader.Close.
func (r *reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true
	r.logger.Info("reader closed")
	return nil
}

// readWithRetry reads a file with retry logic.
func (r *reader) readWithRetry(ctx context.Context, path string, offset int64) ([]parser.UsageEntry, int64, error) {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff.
			backoffMultiplier := 1 << (attempt - 1) // nolint:gosec // Attempt is bounded by MaxRetries
			delay := r.config.RetryDelay * time.Duration(backoffMultiplier)
			r.logger.Debug("retrying read",
				"path", path,
				"attempt", attempt,
				"delay", delay)

			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(delay):
			}
		}

		entries, newOffset, err := r.readFile(ctx, path, offset)
		if err == nil {
			return entries, newOffset, nil
		}

		lastErr = err

		// Check if error is retryable.
		if !r.isRetryable(err) {
			r.logger.Debug("non-retryable error",
				"path", path,
				"error", err)
			return nil, 0, err
		}

		r.logger.Warn("read attempt failed",
			"path", path,
			"attempt", attempt,
			"error", err)
	}

	return nil, 0, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// readFile reads a file from the specified offset.
func (r *reader) readFile(ctx context.Context, path string, offset int64) ([]parser.UsageEntry, int64, error) {
	// Check context before opening file.
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}

	// Get file info.
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, ErrFileNotFound
		}
		if os.IsPermission(err) {
			return nil, 0, ErrPermissionDenied
		}
		return nil, 0, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check file size.
	fileSize := info.Size()
	if fileSize > r.config.MaxFileSize {
		return nil, 0, ErrFileTooLarge
	}

	// Check if file was truncated.
	if offset > fileSize {
		r.logger.Warn("file was truncated, resetting offset",
			"path", path,
			"old_offset", offset,
			"file_size", fileSize)
		offset = 0
	}

	// If offset equals file size, no new data.
	if offset == fileSize {
		return []parser.UsageEntry{}, offset, nil
	}

	// Parse file from offset.
	entries, newOffset, err := r.parser.ParseFile(path, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse file: %w", err)
	}

	return entries, newOffset, nil
}

// isRetryable checks if an error is retryable.
func (r *reader) isRetryable(err error) bool {
	switch err {
	case ErrFileLocked:
		return true
	case ErrFileNotFound:
		return true // File might be created shortly.
	case ErrPermissionDenied:
		return false
	case ErrFileTooLarge:
		return false
	case ErrInvalidOffset:
		return false
	case context.Canceled:
		return false
	case context.DeadlineExceeded:
		return false
	default:
		// Retry unknown errors.
		return true
	}
}
