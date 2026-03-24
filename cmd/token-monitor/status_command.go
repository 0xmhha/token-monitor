package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
	"github.com/0xmhha/token-monitor/pkg/config"
	"github.com/0xmhha/token-monitor/pkg/discovery"
	"github.com/0xmhha/token-monitor/pkg/display"
	"github.com/0xmhha/token-monitor/pkg/logger"
	"github.com/0xmhha/token-monitor/pkg/parser"
	"github.com/0xmhha/token-monitor/pkg/reader"
)

// statusCommand outputs a compact, formatted status line for Claude Code's status display.
type statusCommand struct {
	current    bool
	sessionID  string
	compact    bool
	full       bool
	noEmoji    bool
	watch      bool
	interval   time.Duration
	globalOpts globalOptions
}

// statusData holds the aggregated data needed for formatting.
type statusData struct {
	totalTokens  int
	inputTokens  int
	outputTokens int
	ratePerMin   float64
	blockRemain  time.Duration
}

// Execute runs the status command.
func (c *statusCommand) Execute() error {
	if c.watch {
		return c.runWatch()
	}
	return c.printOnce()
}

// printOnce collects data and prints a single status line.
func (c *statusCommand) printOnce() error {
	data, err := c.collect()
	if err != nil {
		return err
	}
	fmt.Println(c.format(data))
	return nil
}

// runWatch loops at the configured interval, printing status lines.
func (c *statusCommand) runWatch() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Print first line immediately.
	c.printWatchLine(isTerminal, true)

	for {
		select {
		case <-sigChan:
			if isTerminal {
				fmt.Println()
			}
			return nil
		case <-ticker.C:
			c.printWatchLine(isTerminal, false)
		}
	}
}

// printWatchLine prints one status line, overwriting previous if terminal.
func (c *statusCommand) printWatchLine(isTerminal bool, first bool) {
	data, err := c.collect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "status: %v\n", err)
		return
	}
	line := c.format(data)
	if isTerminal && !first {
		fmt.Printf("\r\033[K%s", line)
	} else {
		fmt.Println(line)
	}
}

// collect resolves the target session(s) and aggregates token data.
func (c *statusCommand) collect() (statusData, error) {
	cfg, err := config.Load()
	if err != nil {
		return statusData{}, fmt.Errorf("failed to load config: %w", err)
	}

	log := c.buildLogger(cfg)
	disc := discovery.New(cfg.ClaudeConfigDirs, log)

	sessions, err := c.resolveSessions(disc)
	if err != nil {
		return statusData{}, err
	}

	posStore := reader.NewMemoryPositionStore()
	r, err := reader.New(reader.Config{
		PositionStore: posStore,
		Parser:        parser.New(),
	}, log)
	if err != nil {
		return statusData{}, fmt.Errorf("failed to create reader: %w", err)
	}
	defer r.Close() //nolint:errcheck

	agg := aggregator.New(aggregator.Config{})
	ctx := context.Background()

	for _, sess := range sessions {
		entries, _, readErr := r.ReadFrom(ctx, sess.FilePath, 0)
		if readErr != nil {
			log.Warn("failed to read session", "session", sess.SessionID, "error", readErr)
			continue
		}
		for _, entry := range entries {
			agg.Add(entry)
		}
	}

	stats := agg.Stats()
	burnRate := agg.BurnRate("", 5*time.Minute)
	block := agg.CurrentBillingBlock("")

	remaining := time.Duration(0)
	if block.IsActive {
		remaining = time.Until(block.EndTime)
		if remaining < 0 {
			remaining = 0
		}
	}

	return statusData{
		totalTokens:  stats.TotalTokens,
		inputTokens:  stats.InputTokens,
		outputTokens: stats.OutputTokens,
		ratePerMin:   burnRate.TokensPerMinute,
		blockRemain:  remaining,
	}, nil
}

// resolveSessions returns the session files to aggregate.
func (c *statusCommand) resolveSessions(disc discovery.Discoverer) ([]discovery.SessionFile, error) {
	if c.sessionID != "" {
		sessions, err := disc.Discover()
		if err != nil {
			return nil, fmt.Errorf("failed to discover sessions: %w", err)
		}
		filtered := make([]discovery.SessionFile, 0, 1)
		for _, s := range sessions {
			if s.SessionID == c.sessionID {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("session not found: %s", c.sessionID)
		}
		return filtered, nil
	}

	if c.current {
		sess, err := disc.FindCurrentSession()
		if err != nil {
			return nil, fmt.Errorf("failed to find current session: %w", err)
		}
		return []discovery.SessionFile{*sess}, nil
	}

	sessions, err := disc.Discover()
	if err != nil {
		return nil, fmt.Errorf("failed to discover sessions: %w", err)
	}
	return sessions, nil
}

// format renders data into the requested output format string.
func (c *statusCommand) format(d statusData) string {
	switch {
	case c.compact:
		return c.formatCompact(d)
	case c.full:
		return c.formatFull(d)
	default:
		return c.formatDefault(d)
	}
}

// formatCompact renders a minimal format of approximately 13 chars.
func (c *statusCommand) formatCompact(d statusData) string {
	total := display.FormatCompact(d.totalTokens)
	rate := display.FormatCompact(int(d.ratePerMin))
	return fmt.Sprintf("%s/%s↑", total, rate)
}

// formatDefault renders the standard format of approximately 45 chars.
func (c *statusCommand) formatDefault(d statusData) string {
	total := display.FormatCompact(d.totalTokens)
	rate := display.FormatCompact(int(d.ratePerMin))
	remain := display.FormatDuration(d.blockRemain)

	if c.noEmoji {
		return fmt.Sprintf("%s tokens | %s/min | Block: %s", total, rate, remain)
	}
	return fmt.Sprintf("🔥 %s tokens | %s/min | Block: %s", total, rate, remain)
}

// formatFull renders the verbose format of approximately 75 chars.
func (c *statusCommand) formatFull(d statusData) string {
	total := display.FormatTokenCount(d.totalTokens)
	rate := display.FormatTokenCount(int(d.ratePerMin))
	in := display.FormatTokenCount(d.inputTokens)
	out := display.FormatTokenCount(d.outputTokens)
	remain := display.FormatDuration(d.blockRemain)
	return fmt.Sprintf("Total: %s | Rate: %s/min | In: %s Out: %s | Block: %s left",
		total, rate, in, out, remain)
}

// buildLogger creates a quiet logger suitable for status output.
func (c *statusCommand) buildLogger(cfg *config.Config) logger.Logger {
	level := "error"
	if c.globalOpts.logLevel != "" {
		level = c.globalOpts.logLevel
	}
	return logger.New(logger.Config{
		Level:  level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})
}

// runStatusCommand parses flags and runs the status command.
func runStatusCommand(globalOpts globalOptions, args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	current := fs.Bool("current", false, "auto-detect current session")
	sessionID := fs.String("session", "", "specify session ID")
	compact := fs.Bool("compact", false, "minimal format (~13 chars)")
	full := fs.Bool("full", false, "verbose format (~75 chars)")
	noEmoji := fs.Bool("no-emoji", false, "omit emoji from output")
	watch := fs.Bool("watch", false, "continuous output mode")
	interval := fs.Duration("interval", 5*time.Second, "watch refresh interval")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cmd := &statusCommand{
		current:    *current,
		sessionID:  *sessionID,
		compact:    *compact,
		full:       *full,
		noEmoji:    *noEmoji,
		watch:      *watch,
		interval:   *interval,
		globalOpts: globalOpts,
	}

	return cmd.Execute()
}
