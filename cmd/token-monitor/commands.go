package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yourusername/token-monitor/pkg/aggregator"
	"github.com/yourusername/token-monitor/pkg/config"
	"github.com/yourusername/token-monitor/pkg/discovery"
	"github.com/yourusername/token-monitor/pkg/display"
	"github.com/yourusername/token-monitor/pkg/logger"
	"github.com/yourusername/token-monitor/pkg/monitor"
	"github.com/yourusername/token-monitor/pkg/parser"
	"github.com/yourusername/token-monitor/pkg/reader"
	"github.com/yourusername/token-monitor/pkg/session"
	"github.com/yourusername/token-monitor/pkg/watcher"
)

// statsCommand displays token usage statistics.
type statsCommand struct {
	sessionID  string
	model      string
	groupBy    []string
	topN       int
	format     string
	compact    bool
	configPath string
}

// Execute runs the stats command.
func (c *statsCommand) Execute() error {
	// Load configuration and initialize components.
	cfg, log, sessionMgr, r, err := c.initialize()
	if err != nil {
		return err
	}
	defer c.cleanup(sessionMgr, r, log)

	// Discover and collect data.
	agg, err := c.collectStats(cfg, log, r)
	if err != nil {
		return err
	}

	// Display results.
	return c.displayResults(agg)
}

// initialize sets up configuration and components.
func (c *statsCommand) initialize() (*config.Config, logger.Logger, session.Manager, reader.Reader, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	log := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	sessionMgr, err := session.New(session.Config{
		DBPath: cfg.Storage.DBPath,
	}, log)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to initialize session manager: %w", err)
	}

	positionStore, err := reader.NewBoltPositionStore(sessionMgr.DB())
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to initialize position store: %w", err)
	}

	r, err := reader.New(reader.Config{
		PositionStore: positionStore,
		Parser:        parser.New(),
	}, log)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to initialize reader: %w", err)
	}

	return cfg, log, sessionMgr, r, nil
}

// cleanup closes resources.
func (c *statsCommand) cleanup(sessionMgr session.Manager, r reader.Reader, log logger.Logger) {
	if r != nil {
		if err := r.Close(); err != nil {
			log.Error("failed to close reader", "error", err)
		}
	}
	if sessionMgr != nil {
		if err := sessionMgr.Close(); err != nil {
			log.Error("failed to close session manager", "error", err)
		}
	}
}

// collectStats discovers sessions and aggregates statistics.
func (c *statsCommand) collectStats(cfg *config.Config, log logger.Logger, r reader.Reader) (aggregator.Aggregator, error) {
	disc := discovery.New(cfg.ClaudeConfigDirs, log)
	sessions, err := disc.Discover()
	if err != nil {
		return nil, fmt.Errorf("failed to discover sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No session files found")
		return nil, nil
	}

	dimensions, err := c.parseDimensions()
	if err != nil {
		return nil, err
	}

	agg := aggregator.New(aggregator.Config{
		GroupBy:          dimensions,
		TrackPercentiles: true,
	})

	ctx := context.Background()
	for _, sess := range sessions {
		if c.sessionID != "" && sess.SessionID != c.sessionID {
			continue
		}

		entries, readErr := r.Read(ctx, sess.FilePath)
		if readErr != nil {
			log.Warn("failed to read session",
				"session", sess.SessionID,
				"path", sess.FilePath,
				"error", readErr)
			continue
		}

		for _, entry := range entries {
			if c.model != "" && entry.Message.Model != c.model {
				continue
			}
			agg.Add(entry)
		}
	}

	return agg, nil
}

// parseDimensions converts dimension strings to types.
func (c *statsCommand) parseDimensions() ([]aggregator.Dimension, error) {
	var dimensions []aggregator.Dimension
	for _, dim := range c.groupBy {
		switch dim {
		case "model":
			dimensions = append(dimensions, aggregator.DimModel)
		case "session":
			dimensions = append(dimensions, aggregator.DimSession)
		case "date":
			dimensions = append(dimensions, aggregator.DimDate)
		case "hour":
			dimensions = append(dimensions, aggregator.DimHour)
		default:
			return nil, fmt.Errorf("invalid dimension: %s", dim)
		}
	}
	return dimensions, nil
}

// displayResults formats and prints statistics.
func (c *statsCommand) displayResults(agg aggregator.Aggregator) error {
	if agg == nil {
		return nil
	}

	var fmt display.Format
	switch c.format {
	case "json":
		fmt = display.FormatJSON
	case "simple":
		fmt = display.FormatSimple
	default:
		fmt = display.FormatTable
	}

	formatter := display.New(display.Config{
		Format:          fmt,
		ShowPercentiles: true,
		ShowTimestamps:  true,
		Compact:         c.compact,
	})

	if c.topN > 0 {
		topSessions := agg.TopSessions(c.topN)
		return formatter.FormatTopSessions(os.Stdout, topSessions)
	}

	dimensions, err := c.parseDimensions()
	if err != nil {
		return err
	}

	if len(dimensions) > 0 {
		grouped := agg.GroupedStats()
		return formatter.FormatGroupedStats(os.Stdout, grouped, c.groupBy)
	}

	stats := agg.Stats()
	return formatter.FormatStats(os.Stdout, stats)
}

// listCommand lists all discovered sessions.
type listCommand struct {
	configPath string
}

// Execute runs the list command.
func (c *listCommand) Execute() error {
	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger.
	log := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	// Discover session files.
	disc := discovery.New(cfg.ClaudeConfigDirs, log)
	sessions, err := disc.Discover()
	if err != nil {
		return fmt.Errorf("failed to discover sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No session files found")
		return nil
	}

	// Display sessions.
	fmt.Printf("Found %d session(s):\n\n", len(sessions))
	for _, sess := range sessions {
		fmt.Printf("  %s\n", sess.SessionID)
		fmt.Printf("    Path: %s\n", sess.FilePath)
		if sess.ProjectPath != "" {
			fmt.Printf("    Project: %s\n", sess.ProjectPath)
		}
		fmt.Println()
	}

	return nil
}

// watchCommand provides live token usage monitoring.
type watchCommand struct {
	sessionID   string
	refresh     time.Duration
	format      string
	clearScreen bool
	configPath  string
}

// Execute runs the watch command.
func (c *watchCommand) Execute() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger (quiet mode for live monitoring)
	log := logger.New(logger.Config{
		Level:  "error", // Only show errors during live monitoring
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	// Initialize session manager
	sessionMgr, err := session.New(session.Config{
		DBPath: cfg.Storage.DBPath,
	}, log)
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}
	defer func() {
		if err := sessionMgr.Close(); err != nil {
			log.Error("failed to close session manager", "error", err)
		}
	}()

	// Initialize position store and reader
	positionStore, err := reader.NewBoltPositionStore(sessionMgr.DB())
	if err != nil {
		return fmt.Errorf("failed to initialize position store: %w", err)
	}

	r, err := reader.New(reader.Config{
		PositionStore: positionStore,
		Parser:        parser.New(),
	}, log)
	if err != nil {
		return fmt.Errorf("failed to initialize reader: %w", err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			log.Error("failed to close reader", "error", err)
		}
	}()

	// Initialize watcher
	w, err := watcher.New(watcher.Config{
		DebounceInterval: 100 * time.Millisecond,
	}, log)
	if err != nil {
		return fmt.Errorf("failed to initialize watcher: %w", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			log.Error("failed to close watcher", "error", err)
		}
	}()

	// Initialize discovery
	disc := discovery.New(cfg.ClaudeConfigDirs, log)

	// Build session filter
	var sessionIDs []string
	if c.sessionID != "" {
		sessionIDs = []string{c.sessionID}
	}

	// Create monitor
	mon, err := monitor.New(monitor.Config{
		SessionIDs:      sessionIDs,
		RefreshInterval: c.refresh,
		ClearScreen:     c.clearScreen,
	}, w, r, disc, log)
	if err != nil {
		return fmt.Errorf("failed to create monitor: %w", err)
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start monitor in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := mon.Start(); err != nil {
			errChan <- err
		}
	}()

	// Type assertion to access Updates method
	liveMonitor, ok := mon.(interface{ Updates() <-chan monitor.Update })
	if !ok {
		return fmt.Errorf("monitor does not support updates channel")
	}

	// Clear screen and display initial header
	if c.clearScreen {
		fmt.Print("\033[2J\033[H")
	}

	fmt.Println("🔍 Live Token Monitor - Press Ctrl+C to stop")
	if c.sessionID != "" {
		fmt.Printf("Session: %s | ", c.sessionID)
	} else {
		fmt.Print("All Sessions | ")
	}
	fmt.Printf("Refresh: %s\n", c.refresh)
	fmt.Println(strings.Repeat("─", 80))
	fmt.Println()

	// Process updates
	for {
		select {
		case <-sigChan:
			// Move cursor down and print exit message
			fmt.Print("\n\n")
			fmt.Println("Stopping monitor...")
			if err := mon.Stop(); err != nil {
				log.Error("failed to stop monitor", "error", err)
			}
			return nil

		case err := <-errChan:
			return fmt.Errorf("monitor error: %w", err)

		case update := <-liveMonitor.Updates():
			c.displayUpdate(update)
		}
	}
}

// displayUpdate renders a live monitoring update.
func (c *watchCommand) displayUpdate(update monitor.Update) {
	// Save cursor position, move to line 5 (after header), and clear from there
	if c.clearScreen {
		// Move cursor to line 5 (after header) and clear from cursor to end
		fmt.Print("\033[5;1H\033[J")
	}

	// Format based on configured format
	switch c.format {
	case "simple":
		c.displaySimple(update)
	default:
		c.displayTable(update)
	}
}

// displaySimple shows a simple text format.
func (c *watchCommand) displaySimple(update monitor.Update) {
	stats := update.Stats
	delta := update.Delta
	cumulative := update.Cumulative

	fmt.Printf("📊 Token Usage Statistics (Last updated: %s)\n\n",
		update.Timestamp.Format("15:04:05"))

	fmt.Printf("Total Requests:  %d (session: %+d, now: %+d)\n",
		stats.Count, cumulative.NewEntries, delta.NewEntries)
	fmt.Printf("Input Tokens:    %d (session: %+d, now: %+d)\n",
		stats.InputTokens, cumulative.InputTokens, delta.InputTokens)
	fmt.Printf("Output Tokens:   %d (session: %+d, now: %+d)\n",
		stats.OutputTokens, cumulative.OutputTokens, delta.OutputTokens)
	fmt.Printf("Total Tokens:    %d (session: %+d, now: %+d)\n",
		stats.TotalTokens, cumulative.TotalTokens, delta.TotalTokens)

	fmt.Println()
	fmt.Printf("Average/Request: %.0f\n", stats.AvgTokens)
	fmt.Printf("Min Tokens:      %d\n", stats.MinTokens)
	fmt.Printf("Max Tokens:      %d\n", stats.MaxTokens)

	if stats.P50Tokens > 0 {
		fmt.Printf("P50 Tokens:      %d\n", stats.P50Tokens)
		fmt.Printf("P95 Tokens:      %d\n", stats.P95Tokens)
		fmt.Printf("P99 Tokens:      %d\n", stats.P99Tokens)
	}

	if !stats.FirstSeen.IsZero() {
		fmt.Printf("\nFirst Activity:  %s\n", stats.FirstSeen.Format("2006-01-02 15:04:05"))
		fmt.Printf("Last Activity:   %s\n", stats.LastSeen.Format("2006-01-02 15:04:05"))
		duration := stats.LastSeen.Sub(stats.FirstSeen)
		if duration > 0 {
			fmt.Printf("Duration:        %s\n", duration.Round(time.Second))
		}
	}
}

// displayTable shows a table format.
func (c *watchCommand) displayTable(update monitor.Update) {
	stats := update.Stats
	delta := update.Delta
	cumulative := update.Cumulative

	fmt.Printf("📊 Live Token Monitor - %s\n\n",
		update.Timestamp.Format("2006-01-02 15:04:05"))

	// Token counts table with session cumulative and real-time delta
	fmt.Println("┌─────────────────┬──────────────┬──────────────┬────────────┐")
	fmt.Println("│ Metric          │ Total        │ Session +    │ Now +      │")
	fmt.Println("├─────────────────┼──────────────┼──────────────┼────────────┤")
	fmt.Printf("│ Requests        │ %12d │ %+12d │ %+10d │\n", stats.Count, cumulative.NewEntries, delta.NewEntries)
	fmt.Printf("│ Input Tokens    │ %12d │ %+12d │ %+10d │\n", stats.InputTokens, cumulative.InputTokens, delta.InputTokens)
	fmt.Printf("│ Output Tokens   │ %12d │ %+12d │ %+10d │\n", stats.OutputTokens, cumulative.OutputTokens, delta.OutputTokens)
	fmt.Printf("│ Total Tokens    │ %12d │ %+12d │ %+10d │\n", stats.TotalTokens, cumulative.TotalTokens, delta.TotalTokens)
	fmt.Println("└─────────────────┴──────────────┴──────────────┴────────────┘")

	// Statistics table
	fmt.Println()
	fmt.Println("┌─────────────────┬──────────────┐")
	fmt.Println("│ Statistic       │ Value        │")
	fmt.Println("├─────────────────┼──────────────┤")
	fmt.Printf("│ Average         │ %12.0f │\n", stats.AvgTokens)
	fmt.Printf("│ Min             │ %12d │\n", stats.MinTokens)
	fmt.Printf("│ Max             │ %12d │\n", stats.MaxTokens)

	if stats.P50Tokens > 0 {
		fmt.Printf("│ P50             │ %12d │\n", stats.P50Tokens)
		fmt.Printf("│ P95             │ %12d │\n", stats.P95Tokens)
		fmt.Printf("│ P99             │ %12d │\n", stats.P99Tokens)
	}
	fmt.Println("└─────────────────┴──────────────┘")

	// Activity timeline
	if !stats.FirstSeen.IsZero() {
		fmt.Println()
		fmt.Printf("⏱️  First: %s | Last: %s",
			stats.FirstSeen.Format("15:04:05"),
			stats.LastSeen.Format("15:04:05"))

		duration := stats.LastSeen.Sub(stats.FirstSeen)
		if duration > 0 {
			fmt.Printf(" | Duration: %s", duration.Round(time.Second))
		}
		fmt.Println()
	}
}
