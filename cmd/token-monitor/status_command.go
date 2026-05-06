package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
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
	fromStdin  bool   // --from-stdin: read Claude Code's statusline JSON envelope from stdin
	breakdown  bool   // --breakdown: emit cross-session day:total | model:total compact line
	window     string // --window: today, all, Nd, Nh (default "today")
	modelGlob  string // --model-glob: filter by model glob, e.g. "*sonnet*"
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
	// Reading stdin is only meaningful when Claude Code is invoking us
	// with its statusline JSON envelope. The session_id it provides
	// pins single-session output to the exact session the user is
	// currently in; without it, we fall through to discovery's
	// "most recently modified" heuristic.
	//
	// For the cross-session --breakdown mode the session_id is NOT used
	// to filter (the whole point of breakdown is to sum across every
	// session in the window) — but we still drain stdin so Claude Code's
	// statusline pipe doesn't see a SIGPIPE-style oddity on its end.
	if c.fromStdin {
		in := readStatuslineInput()
		if !c.breakdown && in != nil && in.SessionID != "" && c.sessionID == "" {
			c.sessionID = in.SessionID
		}
	}

	if c.breakdown {
		return c.printBreakdown()
	}
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

// printBreakdown collects entries from all (or one) session, applies the
// time window and model glob filters, and prints a single compact line
// like:
//
//	day:340K | son:128K | opus:212K
//
// The total ("day" or whatever the window header) comes first, followed by
// per-model abbreviations sorted alphabetically.
func (c *statusCommand) printBreakdown() error {
	entries, err := c.collectEntries()
	if err != nil {
		return err
	}

	now := time.Now()
	since, err := display.ParseWindow(c.window, now)
	if err != nil {
		return err
	}

	// FilterSince treats a zero cutoff as "include all" (window=all),
	// so we can pass `since` through unconditionally.
	filtered := aggregator.FilterSince(entries, since)
	filtered = aggregator.FilterByModelGlob(filtered, c.modelGlob)
	breakdown := aggregator.BreakdownByModel(filtered)

	fmt.Println(formatBreakdown(c.window, breakdown))
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

// collectEntries resolves the target session(s) and returns the raw
// usage entries without aggregation. The breakdown path needs the raw
// stream so it can apply window + glob filters before grouping.
//
// When sessionID is empty (typical breakdown case: "all sessions today"),
// this discovers every session under the configured Claude config dirs.
// Read errors on individual sessions are logged and skipped, matching
// collect()'s tolerance.
func (c *statusCommand) collectEntries() ([]parser.UsageEntry, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	log := c.buildLogger(cfg)
	disc := discovery.New(cfg.ClaudeConfigDirs, log)

	sessions, err := c.resolveSessions(disc)
	if err != nil {
		return nil, err
	}

	posStore := reader.NewMemoryPositionStore()
	r, err := reader.New(reader.Config{
		PositionStore: posStore,
		Parser:        parser.New(),
	}, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer r.Close() //nolint:errcheck

	ctx := context.Background()
	all := make([]parser.UsageEntry, 0, 1024)
	for _, sess := range sessions {
		entries, _, readErr := r.ReadFrom(ctx, sess.FilePath, 0)
		if readErr != nil {
			log.Warn("failed to read session", "session", sess.SessionID, "error", readErr)
			continue
		}
		all = append(all, entries...)
	}
	return all, nil
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
	fromStdin := fs.Bool("from-stdin", false, "read Claude Code statusline JSON envelope from stdin")
	breakdown := fs.Bool("breakdown", false, "emit cross-session breakdown line (day:total | model:total)")
	window := fs.String("window", "today", "time window for --breakdown: today, all, Nd, or Nh")
	modelGlob := fs.String("model-glob", "", "filter --breakdown by model glob, e.g. '*sonnet*'")

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
		fromStdin:  *fromStdin,
		breakdown:  *breakdown,
		window:     *window,
		modelGlob:  *modelGlob,
		globalOpts: globalOpts,
	}

	return cmd.Execute()
}

// formatBreakdown renders a single compact line:
//
//	<window-label>:<total> | <abbr1>:<total1> | <abbr2>:<total2>
//
// The window label is derived from the window string ("today" → "day",
// "7d" → "7d", "all" → "all", "" → "day"). Per-model entries are sorted
// alphabetically by abbreviation for deterministic output.
//
// Distinct exact-model identifiers that share an abbreviation are merged
// (e.g. claude-opus-4-7 + claude-opus-4-1 → one "opus:" row). This keeps
// the compact line readable when several minor versions of the same
// family ran in the window.
//
// When the breakdown map is empty (no entries matched the window or
// glob), the line collapses to "<label>:0".
func formatBreakdown(window string, breakdown map[string]aggregator.ModelBreakdown) string {
	total := 0
	byAbbr := make(map[string]int, len(breakdown))
	for _, b := range breakdown {
		total += b.TotalTokens
		byAbbr[abbreviateModel(b.Model)] += b.TotalTokens
	}

	abbrs := make([]string, 0, len(byAbbr))
	for a := range byAbbr {
		abbrs = append(abbrs, a)
	}
	sort.Strings(abbrs)

	var sb strings.Builder
	sb.WriteString(windowLabel(window))
	sb.WriteByte(':')
	sb.WriteString(display.FormatCompact(total))
	for _, a := range abbrs {
		sb.WriteString(" | ")
		sb.WriteString(a)
		sb.WriteByte(':')
		sb.WriteString(display.FormatCompact(byAbbr[a]))
	}
	return sb.String()
}

// windowLabel maps the window flag to a short display label.
// "today" / "" → "day"; everything else passes through trimmed and lowercased.
func windowLabel(window string) string {
	w := strings.ToLower(strings.TrimSpace(window))
	if w == "" || w == "today" {
		return "day"
	}
	return w
}

// abbreviateModel collapses a model identifier to a 3-4 character tag.
// Recognized substrings win regardless of position:
//
//	"*sonnet*" → "son"
//	"*opus*"   → "opus"
//	"*haiku*"  → "hai"
//
// Otherwise the first up-to-4 characters of the model are used. The
// abbreviation is always lowercase so sorting in formatBreakdown is
// stable across mixed-case model names.
func abbreviateModel(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "sonnet"):
		return "son"
	case strings.Contains(m, "opus"):
		return "opus"
	case strings.Contains(m, "haiku"):
		return "hai"
	}
	if len(m) > 4 {
		return m[:4]
	}
	return m
}
