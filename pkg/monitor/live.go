package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/token-monitor/pkg/aggregator"
	"github.com/yourusername/token-monitor/pkg/discovery"
	"github.com/yourusername/token-monitor/pkg/logger"
	"github.com/yourusername/token-monitor/pkg/reader"
	"github.com/yourusername/token-monitor/pkg/watcher"
)

// liveMonitor implements the LiveMonitor interface.
type liveMonitor struct {
	config    Config
	logger    logger.Logger
	watcher   watcher.Watcher
	reader    reader.Reader
	discovery discovery.Discoverer

	mu       sync.RWMutex
	running  bool
	closed   bool
	stopChan chan struct{}

	// Aggregation state
	agg       aggregator.Aggregator
	lastStats aggregator.Statistics

	// Update channel for consumers
	updates chan Update

	// Session file paths being monitored
	sessionPaths map[string]string // sessionID -> filePath
}

// New creates a new live monitor.
//
// Parameters:
//   - cfg: Monitor configuration
//   - w: File watcher
//   - r: Incremental reader
//   - disc: Session discovery
//   - log: Logger instance
//
// Returns:
//   - Configured LiveMonitor
//   - Error if configuration is invalid
func New(cfg Config, w watcher.Watcher, r reader.Reader, disc discovery.Discoverer, log logger.Logger) (LiveMonitor, error) {
	// Validate configuration
	if cfg.RefreshInterval == 0 {
		cfg.RefreshInterval = time.Second
	}

	m := &liveMonitor{
		config:       cfg,
		logger:       log,
		watcher:      w,
		reader:       r,
		discovery:    disc,
		stopChan:     make(chan struct{}),
		updates:      make(chan Update, 10),
		sessionPaths: make(map[string]string),
		agg: aggregator.New(aggregator.Config{
			TrackPercentiles: true,
		}),
	}

	log.Info("live monitor created",
		"refresh_interval", cfg.RefreshInterval,
		"session_filter", cfg.SessionIDs)

	return m, nil
}

// Start implements LiveMonitor.Start.
func (m *liveMonitor) Start() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrMonitorClosed
	}
	if m.running {
		m.mu.Unlock()
		return ErrMonitorRunning
	}
	m.running = true
	m.mu.Unlock()

	// Discover sessions
	sessions, err := m.discovery.Discover()
	if err != nil {
		return fmt.Errorf("failed to discover sessions: %w", err)
	}

	// Filter sessions if specified
	filteredSessions := m.filterSessions(sessions)
	if len(filteredSessions) == 0 {
		return ErrNoSessions
	}

	// Build session path map and watch paths
	watchPaths := make([]string, 0, len(filteredSessions))
	for _, sess := range filteredSessions {
		m.sessionPaths[sess.SessionID] = sess.FilePath
		watchPaths = append(watchPaths, sess.FilePath)
	}

	m.logger.Info("monitoring sessions",
		"count", len(filteredSessions),
		"sessions", m.config.SessionIDs)

	// Initial read of all session files
	ctx := context.Background()
	if err := m.initialRead(ctx); err != nil {
		return fmt.Errorf("initial read failed: %w", err)
	}

	// Start file watcher
	if err := m.watcher.Start(ctx, watchPaths); err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}

	// Start event processing
	go m.processEvents(ctx)

	// Start periodic updates
	go m.periodicUpdates()

	m.logger.Info("live monitor started")
	return nil
}

// Stop implements LiveMonitor.Stop.
func (m *liveMonitor) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrMonitorClosed
	}
	if !m.running {
		return ErrMonitorNotRunning
	}

	// Signal stop
	close(m.stopChan)
	m.running = false

	// Stop watcher
	if err := m.watcher.Stop(); err != nil {
		m.logger.Warn("failed to stop watcher", "error", err)
	}

	m.logger.Info("live monitor stopped")
	return nil
}

// Stats implements LiveMonitor.Stats.
func (m *liveMonitor) Stats() aggregator.Statistics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.agg.Stats()
}

// Updates returns a channel for receiving live updates.
func (m *liveMonitor) Updates() <-chan Update {
	return m.updates
}

// filterSessions filters sessions based on configuration.
func (m *liveMonitor) filterSessions(sessions []discovery.SessionFile) []discovery.SessionFile {
	if len(m.config.SessionIDs) == 0 {
		return sessions
	}

	// Build session ID set for quick lookup
	sessionSet := make(map[string]bool)
	for _, id := range m.config.SessionIDs {
		sessionSet[id] = true
	}

	// Filter sessions
	filtered := make([]discovery.SessionFile, 0)
	for _, sess := range sessions {
		if sessionSet[sess.SessionID] {
			filtered = append(filtered, sess)
		}
	}

	return filtered
}

// initialRead reads all session files from the beginning.
func (m *liveMonitor) initialRead(ctx context.Context) error {
	for sessionID, path := range m.sessionPaths {
		entries, err := m.reader.Read(ctx, path)
		if err != nil {
			m.logger.Warn("failed to read session file",
				"session", sessionID,
				"path", path,
				"error", err)
			continue
		}

		// Add entries to aggregator
		for _, entry := range entries {
			m.agg.Add(entry)
		}

		m.logger.Debug("initial read complete",
			"session", sessionID,
			"entries", len(entries))
	}

	// Store initial stats
	m.lastStats = m.agg.Stats()

	return nil
}

// processEvents handles file change events from the watcher.
func (m *liveMonitor) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-m.stopChan:
			return

		case event, ok := <-m.watcher.Events():
			if !ok {
				m.logger.Info("watcher events channel closed")
				return
			}

			m.handleFileChange(ctx, event)

		case err, ok := <-m.watcher.Errors():
			if !ok {
				m.logger.Info("watcher errors channel closed")
				return
			}

			m.logger.Error("watcher error", "error", err)
		}
	}
}

// handleFileChange processes a file change event.
func (m *liveMonitor) handleFileChange(ctx context.Context, event watcher.Event) {
	m.logger.Debug("file change detected",
		"path", event.Path,
		"op", event.Op)

	// Read new entries from the file
	entries, err := m.reader.Read(ctx, event.Path)
	if err != nil {
		m.logger.Warn("failed to read file after change",
			"path", event.Path,
			"error", err)
		return
	}

	if len(entries) == 0 {
		return
	}

	// Add entries to aggregator
	m.mu.Lock()
	for _, entry := range entries {
		m.agg.Add(entry)
	}
	m.mu.Unlock()

	m.logger.Debug("processed file change",
		"path", event.Path,
		"new_entries", len(entries))

	// Trigger immediate update
	m.sendUpdate()
}

// periodicUpdates sends periodic updates even if no file changes.
func (m *liveMonitor) periodicUpdates() {
	ticker := time.NewTicker(m.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return

		case <-ticker.C:
			m.sendUpdate()
		}
	}
}

// sendUpdate sends a statistics update to the updates channel.
func (m *liveMonitor) sendUpdate() {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentStats := m.agg.Stats()

	// Calculate delta
	delta := DeltaStats{
		NewEntries:   currentStats.Count - m.lastStats.Count,
		InputTokens:  currentStats.InputTokens - m.lastStats.InputTokens,
		OutputTokens: currentStats.OutputTokens - m.lastStats.OutputTokens,
		TotalTokens:  currentStats.TotalTokens - m.lastStats.TotalTokens,
	}

	update := Update{
		Timestamp: time.Now(),
		Stats:     currentStats,
		Delta:     delta,
	}

	// Send update (non-blocking)
	select {
	case m.updates <- update:
	default:
		m.logger.Warn("updates channel full, dropping update")
	}

	// Update last stats
	m.lastStats = currentStats
}

// Close closes the monitor and releases resources.
func (m *liveMonitor) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true

	// Stop if running
	if m.running {
		close(m.stopChan)
		m.running = false
	}

	// Close update channel
	close(m.updates)

	m.logger.Info("live monitor closed")
	return nil
}
