package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
	"github.com/0xmhha/token-monitor/pkg/config"
	"github.com/0xmhha/token-monitor/pkg/discovery"
	"github.com/0xmhha/token-monitor/pkg/display"
	"github.com/0xmhha/token-monitor/pkg/logger"
	"github.com/0xmhha/token-monitor/pkg/monitor"
	"github.com/0xmhha/token-monitor/pkg/parser"
	"github.com/0xmhha/token-monitor/pkg/reader"
	"github.com/0xmhha/token-monitor/pkg/session"
	"github.com/0xmhha/token-monitor/pkg/watcher"
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
	globalOpts globalOptions
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

	// Use global log level if set, otherwise use config.
	logLevel := cfg.Logging.Level
	if c.globalOpts.logLevel != "" {
		logLevel = c.globalOpts.logLevel
	}

	log := logger.New(logger.Config{
		Level:  logLevel,
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
	globalOpts globalOptions
}

// Execute runs the list command.
func (c *listCommand) Execute() error {
	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger.
	// Use global log level if set, otherwise use config.
	logLevel := cfg.Logging.Level
	if c.globalOpts.logLevel != "" {
		logLevel = c.globalOpts.logLevel
	}

	log := logger.New(logger.Config{
		Level:  logLevel,
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
	globalOpts  globalOptions

	// Internal state for keyboard handling
	showHelp   bool
	lastUpdate *monitor.Update
}

// watchRuntime holds all runtime dependencies for the watch command.
// This structure enables dependency injection and easier testing.
type watchRuntime struct {
	config     *config.Config
	log        logger.Logger
	sessionMgr session.Manager
	reader     reader.Reader
	watcher    watcher.Watcher
	monitor    monitor.LiveMonitor
}

// Close releases all runtime resources.
func (rt *watchRuntime) Close() {
	if rt.watcher != nil {
		_ = rt.watcher.Close() //nolint:errcheck // best effort cleanup
	}
	if rt.reader != nil {
		_ = rt.reader.Close() //nolint:errcheck // best effort cleanup
	}
	if rt.sessionMgr != nil {
		_ = rt.sessionMgr.Close() //nolint:errcheck // best effort cleanup
	}
}

// Execute runs the watch command.
func (c *watchCommand) Execute() error {
	rt, err := c.initializeRuntime()
	if err != nil {
		return err
	}
	defer rt.Close()

	if err := c.startMonitor(rt); err != nil {
		return err
	}

	return c.runEventLoop(rt)
}

// initializeRuntime creates and configures all required components.
func (c *watchCommand) initializeRuntime() (*watchRuntime, error) {
	rt := &watchRuntime{}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	rt.config = cfg

	rt.log = c.createLogger(cfg)

	if err := c.initializeStorage(rt); err != nil {
		rt.Close()
		return nil, err
	}

	if err := c.initializeWatcher(rt); err != nil {
		rt.Close()
		return nil, err
	}

	if err := c.initializeMonitor(rt); err != nil {
		rt.Close()
		return nil, err
	}

	return rt, nil
}

// createLogger creates a logger with appropriate settings.
func (c *watchCommand) createLogger(cfg *config.Config) logger.Logger {
	logLevel := "error" // Quiet mode for live monitoring by default
	if c.globalOpts.logLevel != "" {
		logLevel = c.globalOpts.logLevel
	}

	return logger.New(logger.Config{
		Level:  logLevel,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})
}

// initializeStorage sets up session manager and reader.
func (c *watchCommand) initializeStorage(rt *watchRuntime) error {
	sessionMgr, err := session.New(session.Config{
		DBPath: rt.config.Storage.DBPath,
	}, rt.log)
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}
	rt.sessionMgr = sessionMgr

	positionStore, err := reader.NewBoltPositionStore(sessionMgr.DB())
	if err != nil {
		return fmt.Errorf("failed to initialize position store: %w", err)
	}

	r, err := reader.New(reader.Config{
		PositionStore: positionStore,
		Parser:        parser.New(),
	}, rt.log)
	if err != nil {
		return fmt.Errorf("failed to initialize reader: %w", err)
	}
	rt.reader = r

	return nil
}

// initializeWatcher sets up the file watcher.
func (c *watchCommand) initializeWatcher(rt *watchRuntime) error {
	w, err := watcher.New(watcher.Config{
		DebounceInterval: 100 * time.Millisecond,
	}, rt.log)
	if err != nil {
		return fmt.Errorf("failed to initialize watcher: %w", err)
	}
	rt.watcher = w
	return nil
}

// initializeMonitor creates the live monitor.
func (c *watchCommand) initializeMonitor(rt *watchRuntime) error {
	disc := discovery.New(rt.config.ClaudeConfigDirs, rt.log)

	var sessionIDs []string
	if c.sessionID != "" {
		sessionIDs = []string{c.sessionID}
	}

	mon, err := monitor.New(monitor.Config{
		SessionIDs:      sessionIDs,
		RefreshInterval: c.refresh,
		ClearScreen:     c.clearScreen,
	}, rt.watcher, rt.reader, disc, rt.log)
	if err != nil {
		return fmt.Errorf("failed to create monitor: %w", err)
	}
	rt.monitor = mon

	return nil
}

// startMonitor begins the monitoring process.
func (c *watchCommand) startMonitor(rt *watchRuntime) error {
	errChan := make(chan error, 1)
	go func() {
		if err := rt.monitor.Start(); err != nil {
			errChan <- err
		}
	}()

	// Give monitor time to start and check for immediate errors
	select {
	case err := <-errChan:
		return fmt.Errorf("monitor error: %w", err)
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// runEventLoop handles signals, keyboard input, and monitor updates.
func (c *watchCommand) runEventLoop(rt *watchRuntime) error {
	sigChan := c.setupSignalHandler()
	keyChan, cleanup := c.setupKeyboardInput()
	if cleanup != nil {
		defer cleanup()
	}

	updatesChan := c.getUpdatesChannel(rt.monitor)
	if updatesChan == nil {
		return fmt.Errorf("monitor does not support updates channel")
	}

	c.displayInitialScreen()

	return c.processEvents(rt, sigChan, keyChan, updatesChan)
}

// setupSignalHandler configures OS signal handling.
func (c *watchCommand) setupSignalHandler() <-chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	return sigChan
}

// setupKeyboardInput configures terminal for raw input mode.
// Returns a channel for key events and a cleanup function.
func (c *watchCommand) setupKeyboardInput() (<-chan byte, func()) {
	keyChan := make(chan byte, 10)

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return keyChan, nil
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return keyChan, nil
	}

	// Start keyboard reader goroutine
	go c.readKeyboardInput(keyChan)

	cleanup := func() {
		_ = term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck
	}

	return keyChan, cleanup
}

// readKeyboardInput reads bytes from stdin and sends to channel.
func (c *watchCommand) readKeyboardInput(keyChan chan<- byte) {
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return
		}
		select {
		case keyChan <- buf[0]:
		default:
		}
	}
}

// getUpdatesChannel extracts the updates channel from the monitor.
func (c *watchCommand) getUpdatesChannel(mon monitor.LiveMonitor) <-chan monitor.Update {
	liveMonitor, ok := mon.(interface{ Updates() <-chan monitor.Update })
	if !ok {
		return nil
	}
	return liveMonitor.Updates()
}

// displayInitialScreen clears and shows the header.
func (c *watchCommand) displayInitialScreen() {
	if c.clearScreen {
		fmt.Print("\033[2J\033[H")
	}
	c.displayHeader()
}

// processEvents is the main event loop.
func (c *watchCommand) processEvents(
	rt *watchRuntime,
	sigChan <-chan os.Signal,
	keyChan <-chan byte,
	updatesChan <-chan monitor.Update,
) error { //nolint:unparam // error return kept for future error handling
	for {
		select {
		case <-sigChan:
			c.handleQuit(rt.monitor, rt.log)
			return nil

		case key := <-keyChan:
			if c.handleKeyPress(key, rt.monitor, rt.log) == "quit" {
				return nil
			}

		case update := <-updatesChan:
			c.handleUpdate(update)
		}
	}
}

// handleUpdate processes a monitor update event.
func (c *watchCommand) handleUpdate(update monitor.Update) {
	c.lastUpdate = &update
	if c.showHelp {
		c.displayHelpOverlay()
	} else {
		c.displayUpdate(update)
	}
}

// displayHeader shows the initial header for the watch command.
func (c *watchCommand) displayHeader() {
	fmt.Println("🔍 Live Token Monitor - Press ? for help, q to quit")
	if c.sessionID != "" {
		fmt.Printf("Session: %s | ", c.sessionID)
	} else {
		fmt.Print("All Sessions | ")
	}
	fmt.Printf("Refresh: %s\n", c.refresh)
	fmt.Println(strings.Repeat("─", 80))
	fmt.Println()
}

// handleKeyPress processes keyboard input and returns an action.
func (c *watchCommand) handleKeyPress(key byte, mon monitor.LiveMonitor, log logger.Logger) string {
	switch key {
	case 'q', 'Q', 3: // 'q', 'Q', or Ctrl+C
		c.handleQuit(mon, log)
		return "quit"

	case 'r', 'R':
		c.handleReset(mon, log)
		return "reset"

	case '?', 'h', 'H':
		c.showHelp = !c.showHelp
		if c.showHelp {
			c.displayHelpOverlay()
		} else if c.lastUpdate != nil {
			// Redraw the stats display
			if c.clearScreen {
				fmt.Print("\033[2J\033[H")
				c.displayHeader()
			}
			c.displayUpdate(*c.lastUpdate)
		}
		return "help"

	case 27: // ESC key - close help overlay
		if c.showHelp {
			c.showHelp = false
			if c.lastUpdate != nil {
				if c.clearScreen {
					fmt.Print("\033[2J\033[H")
					c.displayHeader()
				}
				c.displayUpdate(*c.lastUpdate)
			}
		}
		return "esc"
	}

	return ""
}

// handleQuit gracefully stops the monitor.
func (c *watchCommand) handleQuit(mon monitor.LiveMonitor, log logger.Logger) {
	// Move cursor down and print exit message
	fmt.Print("\n\n")
	fmt.Println("Stopping monitor...")
	if err := mon.Stop(); err != nil {
		log.Error("failed to stop monitor", "error", err)
	}
}

// handleReset resets the monitor statistics.
func (c *watchCommand) handleReset(mon monitor.LiveMonitor, log logger.Logger) {
	// Try to reset using the Resettable interface
	if resettable, ok := mon.(interface{ ResetStats() }); ok {
		resettable.ResetStats()
		log.Info("statistics reset")
	}

	// Show reset confirmation
	if c.clearScreen {
		fmt.Print("\033[2J\033[H")
		c.displayHeader()
	}
	fmt.Println("📊 Statistics have been reset")
	fmt.Println()
}

// displayHelpOverlay shows the keyboard shortcuts help.
func (c *watchCommand) displayHelpOverlay() {
	if c.clearScreen {
		fmt.Print("\033[5;1H\033[J") // Move to line 5 and clear
	}

	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────────────────────┐")
	fmt.Println("│                  Keyboard Shortcuts                     │")
	fmt.Println("├─────────────────────────────────────────────────────────┤")
	fmt.Println("│  q, Q, Ctrl+C    Quit the monitor                       │")
	fmt.Println("│  r, R            Reset statistics                       │")
	fmt.Println("│  ?, h, H         Toggle this help overlay               │")
	fmt.Println("│  ESC             Close this help overlay                │")
	fmt.Println("├─────────────────────────────────────────────────────────┤")
	fmt.Println("│  Press any key to close this help and return to stats   │")
	fmt.Println("└─────────────────────────────────────────────────────────┘")
	fmt.Println()
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

	// Burn rate
	burnRate := update.BurnRate
	if burnRate.EntryCount > 0 {
		fmt.Printf("\n🔥 Burn Rate (5m window)\n")
		fmt.Printf("Tokens/min:      %.1f\n", burnRate.TokensPerMinute)
		fmt.Printf("Tokens/hour:     %.0f (projected)\n", burnRate.TokensPerHour)
		fmt.Printf("Entries:         %d\n", burnRate.EntryCount)
	}

	// Current billing block
	block := update.CurrentBlock
	if block.EntryCount > 0 {
		fmt.Printf("\n📊 Current Billing Block (%s - %s UTC)\n",
			block.StartTime.UTC().Format("15:04"),
			block.EndTime.UTC().Format("15:04"))
		fmt.Printf("Block Tokens:    %d\n", block.TotalTokens)
		fmt.Printf("Block Entries:   %d\n", block.EntryCount)

		// Calculate time remaining in block
		remaining := block.EndTime.Sub(time.Now().UTC())
		if remaining > 0 {
			fmt.Printf("Time Remaining:  %s\n", remaining.Round(time.Minute))
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

	// Burn rate table
	burnRate := update.BurnRate
	if burnRate.EntryCount > 0 {
		fmt.Println()
		fmt.Println("🔥 Burn Rate (5-minute window)")
		fmt.Println("┌─────────────────┬──────────────┐")
		fmt.Println("│ Metric          │ Value        │")
		fmt.Println("├─────────────────┼──────────────┤")
		fmt.Printf("│ Tokens/min      │ %12.1f │\n", burnRate.TokensPerMinute)
		fmt.Printf("│ Tokens/hour     │ %12.0f │\n", burnRate.TokensPerHour)
		fmt.Printf("│ Input/min       │ %12.1f │\n", burnRate.InputTokensPerMinute)
		fmt.Printf("│ Output/min      │ %12.1f │\n", burnRate.OutputTokensPerMinute)
		fmt.Printf("│ Entries         │ %12d │\n", burnRate.EntryCount)
		fmt.Println("└─────────────────┴──────────────┘")
	}

	// Billing block table
	block := update.CurrentBlock
	if block.EntryCount > 0 {
		fmt.Println()
		fmt.Printf("📊 Current Billing Block (%s - %s UTC)\n",
			block.StartTime.UTC().Format("15:04"),
			block.EndTime.UTC().Format("15:04"))
		fmt.Println("┌─────────────────┬──────────────┐")
		fmt.Println("│ Metric          │ Value        │")
		fmt.Println("├─────────────────┼──────────────┤")
		fmt.Printf("│ Total Tokens    │ %12d │\n", block.TotalTokens)
		fmt.Printf("│ Input Tokens    │ %12d │\n", block.InputTokens)
		fmt.Printf("│ Output Tokens   │ %12d │\n", block.OutputTokens)
		fmt.Printf("│ Entries         │ %12d │\n", block.EntryCount)

		// Calculate time remaining in block
		remaining := block.EndTime.Sub(time.Now().UTC())
		if remaining > 0 {
			hours := int(remaining.Hours())
			mins := int(remaining.Minutes()) % 60
			fmt.Printf("│ Time Left       │ %9dh%02dm │\n", hours, mins)
		}
		fmt.Println("└─────────────────┴──────────────┘")
	}
}
