package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
	"github.com/0xmhha/token-monitor/pkg/config"
	"github.com/0xmhha/token-monitor/pkg/discovery"
	"github.com/0xmhha/token-monitor/pkg/display"
	"github.com/0xmhha/token-monitor/pkg/logger"
	"github.com/0xmhha/token-monitor/pkg/parser"
	"github.com/0xmhha/token-monitor/pkg/reader"
)

// validMetrics is the set of accepted --metric values.
var validMetrics = []string{
	"total", "input", "output", "count",
	"burn-rate", "burn-rate-hour",
	"block-remaining", "block-tokens",
}

// validMetricsList returns a comma-separated string of valid metric names.
func validMetricsList() string {
	return strings.Join(validMetrics, ", ")
}

// queryOutput is the JSON shape for --json output.
type queryOutput struct {
	SessionID      string  `json:"session_id"`
	TotalTokens    int     `json:"total_tokens"`
	InputTokens    int     `json:"input_tokens"`
	OutputTokens   int     `json:"output_tokens"`
	Count          int     `json:"count"`
	BurnRate       float64 `json:"burn_rate"`
	BurnRateHour   float64 `json:"burn_rate_hour"`
	BlockRemaining string  `json:"block_remaining"`
	BlockTokens    int     `json:"block_tokens"`
}

// queryCommand provides fast, lightweight metric extraction without BoltDB.
type queryCommand struct {
	current    bool
	sessionID  string
	metric     string
	jsonOutput bool
	format     string
	configPath string
	globalOpts globalOptions
}

// Execute runs the query command.
func (c *queryCommand) Execute() error {
	if !c.current && c.sessionID == "" {
		return fmt.Errorf("specify --current or --session <id>\n  Example: token-monitor query --current --metric total")
	}

	cfg, err := config.NewLoader(c.configPath).Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log := c.buildLogger(cfg)

	sessFile, err := c.resolveSession(cfg, log)
	if err != nil {
		return err
	}

	agg, err := c.parseAndAggregate(sessFile.FilePath, log)
	if err != nil {
		return err
	}

	if c.jsonOutput || c.globalOpts.jsonOutput {
		return c.printJSON(sessFile.SessionID, agg)
	}

	if c.format == "hook" {
		return c.printHook(agg)
	}

	if c.metric == "" {
		return fmt.Errorf("specify --metric <name> or --json\n  Valid metrics: %s", validMetricsList())
	}

	return c.printMetric(c.metric, agg)
}

// buildLogger creates a silent logger suitable for hook use.
func (c *queryCommand) buildLogger(cfg *config.Config) logger.Logger {
	level := "error" // suppress noise during hook execution
	if c.globalOpts.logLevel != "" {
		level = c.globalOpts.logLevel
	}
	return logger.New(logger.Config{
		Level:  level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})
}

// resolveSession returns the session file based on flags.
func (c *queryCommand) resolveSession(cfg *config.Config, log logger.Logger) (*discovery.SessionFile, error) {
	disc := discovery.New(cfg.ClaudeConfigDirs, log)

	if c.current {
		return disc.FindCurrentSession()
	}

	return c.findByID(disc, c.sessionID)
}

// findByID locates a session file matching the given session ID.
func (c *queryCommand) findByID(disc discovery.Discoverer, sessionID string) (*discovery.SessionFile, error) {
	sessions, err := disc.Discover()
	if err != nil {
		return nil, fmt.Errorf("failed to discover sessions: %w", err)
	}

	for i := range sessions {
		if sessions[i].SessionID == sessionID {
			return &sessions[i], nil
		}
	}

	return nil, fmt.Errorf("session not found: %s", sessionID)
}

// parseAndAggregate reads the file and returns a populated aggregator.
func (c *queryCommand) parseAndAggregate(filePath string, log logger.Logger) (aggregator.Aggregator, error) {
	r, err := reader.New(reader.Config{
		PositionStore: reader.NewMemoryPositionStore(),
		Parser:        parser.New(),
	}, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer r.Close() //nolint:errcheck

	entries, err := r.Read(context.Background(), filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	agg := aggregator.New(aggregator.Config{TrackPercentiles: false})
	for _, entry := range entries {
		agg.Add(entry)
	}

	return agg, nil
}

// printMetric outputs a single metric value to stdout.
func (c *queryCommand) printMetric(metric string, agg aggregator.Aggregator) error {
	stats := agg.Stats()
	burnRate := agg.BurnRate("", 5*time.Minute)
	block := agg.CurrentBillingBlock("")
	now := time.Now()

	switch metric {
	case "total":
		fmt.Println(stats.TotalTokens)
	case "input":
		fmt.Println(stats.InputTokens)
	case "output":
		fmt.Println(stats.OutputTokens)
	case "count":
		fmt.Println(stats.Count)
	case "burn-rate":
		fmt.Printf("%.1f\n", burnRate.TokensPerMinute)
	case "burn-rate-hour":
		fmt.Printf("%.1f\n", burnRate.TokensPerHour)
	case "block-remaining":
		remaining := block.EndTime.Sub(now)
		if remaining < 0 {
			remaining = 0
		}
		fmt.Println(display.FormatDuration(remaining))
	case "block-tokens":
		fmt.Println(block.TotalTokens)
	default:
		return fmt.Errorf("invalid metric %q — valid metrics: %s", metric, validMetricsList())
	}

	return nil
}

// printJSON outputs all metrics as a JSON object.
func (c *queryCommand) printJSON(sessionID string, agg aggregator.Aggregator) error {
	stats := agg.Stats()
	burnRate := agg.BurnRate("", 5*time.Minute)
	block := agg.CurrentBillingBlock("")
	now := time.Now()

	remaining := block.EndTime.Sub(now)
	if remaining < 0 {
		remaining = 0
	}

	out := queryOutput{
		SessionID:      sessionID,
		TotalTokens:    stats.TotalTokens,
		InputTokens:    stats.InputTokens,
		OutputTokens:   stats.OutputTokens,
		Count:          stats.Count,
		BurnRate:       burnRate.TokensPerMinute,
		BurnRateHour:   burnRate.TokensPerHour,
		BlockRemaining: display.FormatDuration(remaining),
		BlockTokens:    block.TotalTokens,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// printHook outputs a single compact line for Claude Code hook display.
func (c *queryCommand) printHook(agg aggregator.Aggregator) error {
	stats := agg.Stats()
	burnRate := agg.BurnRate("", 5*time.Minute)
	fmt.Printf("Total: %d | Rate: %.1f/min\n", stats.TotalTokens, burnRate.TokensPerMinute)
	return nil
}
