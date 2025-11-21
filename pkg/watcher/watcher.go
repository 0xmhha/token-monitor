package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/0xmhha/token-monitor/pkg/logger"
)

// watcher implements the Watcher interface using fsnotify.
type watcher struct {
	fsw    *fsnotify.Watcher
	logger logger.Logger
	config Config

	events chan Event
	errors chan error

	mu       sync.RWMutex
	running  bool
	closed   bool
	stopChan chan struct{}

	// Debouncing state.
	debounceTimers map[string]*time.Timer
	debounceMu     sync.Mutex

	// Circuit breaker state.
	failureCount int
	lastFailure  time.Time
}

// New creates a new file system watcher.
//
// Parameters:
//   - cfg: Watcher configuration
//   - log: Logger instance
//
// Returns:
//   - Configured Watcher
//   - Error if watcher cannot be created
func New(cfg Config, log logger.Logger) (Watcher, error) {
	// Set defaults.
	if cfg.DebounceInterval == 0 {
		cfg.DebounceInterval = 100 * time.Millisecond
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = time.Second
	}
	if cfg.CircuitBreakerThreshold == 0 {
		cfg.CircuitBreakerThreshold = 5
	}

	// Create fsnotify watcher.
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	w := &watcher{
		fsw:            fsw,
		logger:         log,
		config:         cfg,
		events:         make(chan Event, 100),
		errors:         make(chan error, 10),
		stopChan:       make(chan struct{}),
		debounceTimers: make(map[string]*time.Timer),
	}

	log.Info("file watcher created",
		"debounce_interval", cfg.DebounceInterval,
		"max_retries", cfg.MaxRetries)

	return w, nil
}

// Start implements Watcher.Start.
func (w *watcher) Start(ctx context.Context, paths []string) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return ErrWatcherClosed
	}
	if w.running {
		w.mu.Unlock()
		return ErrAlreadyStarted
	}
	w.running = true
	w.mu.Unlock()

	// Expand and validate paths.
	expandedPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		expanded := expandHome(path)

		// Check if path exists.
		if _, err := os.Stat(expanded); err != nil {
			if os.IsNotExist(err) {
				w.logger.Warn("watch path does not exist, skipping",
					"path", expanded)
				continue
			}
			return fmt.Errorf("failed to stat path %s: %w", expanded, err)
		}

		expandedPaths = append(expandedPaths, expanded)
	}

	if len(expandedPaths) == 0 {
		return ErrInvalidPath
	}

	// Add paths to watcher.
	for _, path := range expandedPaths {
		if err := w.addPathRecursive(path); err != nil {
			return fmt.Errorf("failed to add path %s: %w", path, err)
		}
	}

	w.logger.Info("watcher started",
		"paths", expandedPaths,
		"path_count", len(expandedPaths))

	// Start event processing loop.
	go w.processEvents(ctx)

	return nil
}

// Stop implements Watcher.Stop.
func (w *watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWatcherClosed
	}
	if !w.running {
		return ErrNotStarted
	}

	// Signal stop.
	close(w.stopChan)
	w.running = false

	w.logger.Info("watcher stopped")
	return nil
}

// Events implements Watcher.Events.
func (w *watcher) Events() <-chan Event {
	return w.events
}

// Errors implements Watcher.Errors.
func (w *watcher) Errors() <-chan error {
	return w.errors
}

// Close implements Watcher.Close.
func (w *watcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true

	// Stop if running.
	if w.running {
		close(w.stopChan)
		w.running = false
	}

	// Close channels.
	close(w.events)
	close(w.errors)

	// Cancel debounce timers.
	w.debounceMu.Lock()
	for _, timer := range w.debounceTimers {
		timer.Stop()
	}
	w.debounceTimers = nil
	w.debounceMu.Unlock()

	// Close fsnotify watcher.
	if err := w.fsw.Close(); err != nil {
		w.logger.Error("failed to close fsnotify watcher", "error", err)
		return fmt.Errorf("failed to close watcher: %w", err)
	}

	w.logger.Info("watcher closed")
	return nil
}

// processEvents handles events from fsnotify.
func (w *watcher) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("event processing stopped", "reason", "context cancelled")
			return

		case <-w.stopChan:
			w.logger.Info("event processing stopped", "reason", "stop signal")
			return

		case event, ok := <-w.fsw.Events:
			if !ok {
				w.logger.Warn("fsnotify events channel closed")
				return
			}

			w.handleEvent(event)

		case err, ok := <-w.fsw.Errors:
			if !ok {
				w.logger.Warn("fsnotify errors channel closed")
				return
			}

			w.handleError(err)
		}
	}
}

// handleEvent processes a single fsnotify event with debouncing.
func (w *watcher) handleEvent(event fsnotify.Event) {
	// Skip non-JSONL files.
	if !strings.HasSuffix(event.Name, ".jsonl") {
		return
	}

	// Convert fsnotify op to our Op type.
	var op Op
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		op = OpCreate
	case event.Op&fsnotify.Write == fsnotify.Write:
		op = OpWrite
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		op = OpRemove
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		op = OpRename
	case event.Op&fsnotify.Chmod == fsnotify.Chmod:
		op = OpChmod
	default:
		w.logger.Debug("unknown fsnotify operation",
			"op", event.Op,
			"path", event.Name)
		return
	}

	// Debounce the event.
	w.debounceEvent(Event{
		Path:      event.Name,
		Op:        op,
		Timestamp: time.Now(),
	})
}

// debounceEvent implements event debouncing.
func (w *watcher) debounceEvent(event Event) {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	// Cancel existing timer for this path.
	if timer, exists := w.debounceTimers[event.Path]; exists {
		timer.Stop()
	}

	// Create new debounce timer.
	w.debounceTimers[event.Path] = time.AfterFunc(w.config.DebounceInterval, func() {
		w.mu.RLock()
		closed := w.closed
		w.mu.RUnlock()

		if !closed {
			w.events <- event
		}

		// Clean up timer.
		w.debounceMu.Lock()
		delete(w.debounceTimers, event.Path)
		w.debounceMu.Unlock()
	})
}

// handleError processes fsnotify errors with circuit breaker pattern.
func (w *watcher) handleError(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.failureCount++
	w.lastFailure = time.Now()

	w.logger.Error("fsnotify error",
		"error", err,
		"failure_count", w.failureCount)

	// Check circuit breaker.
	if w.failureCount >= w.config.CircuitBreakerThreshold {
		w.logger.Error("circuit breaker opened",
			"threshold", w.config.CircuitBreakerThreshold)

		// Send circuit breaker error.
		select {
		case w.errors <- ErrCircuitBreakerOpen:
		default:
			w.logger.Warn("error channel full, dropping error")
		}

		return
	}

	// Send error to channel.
	select {
	case w.errors <- err:
	default:
		w.logger.Warn("error channel full, dropping error")
	}
}

// addPathRecursive adds a path and all subdirectories to the watcher.
func (w *watcher) addPathRecursive(path string) error {
	// Add the path itself.
	if err := w.fsw.Add(path); err != nil {
		return fmt.Errorf("failed to add path: %w", err)
	}

	w.logger.Debug("added watch path", "path", path)

	// Walk subdirectories.
	return filepath.Walk(path, func(subPath string, info os.FileInfo, err error) error {
		if err != nil {
			w.logger.Warn("error walking path",
				"path", subPath,
				"error", err)
			return nil // Skip but continue walking.
		}

		// Skip non-directories.
		if !info.IsDir() {
			return nil
		}

		// Skip the root path (already added).
		if subPath == path {
			return nil
		}

		// Add subdirectory.
		if addErr := w.fsw.Add(subPath); addErr != nil {
			w.logger.Warn("failed to add subdirectory",
				"path", subPath,
				"error", addErr)
			return nil // Skip but continue walking.
		}

		w.logger.Debug("added watch subdirectory", "path", subPath)
		return nil
	})
}

// expandHome expands ~ in file paths to the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if path == "~" {
		return homeDir
	}

	return filepath.Join(homeDir, path[2:])
}
