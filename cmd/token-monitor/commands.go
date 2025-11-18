package main

import (
	"context"
	"fmt"
	"os"

	"github.com/yourusername/token-monitor/pkg/aggregator"
	"github.com/yourusername/token-monitor/pkg/config"
	"github.com/yourusername/token-monitor/pkg/discovery"
	"github.com/yourusername/token-monitor/pkg/display"
	"github.com/yourusername/token-monitor/pkg/logger"
	"github.com/yourusername/token-monitor/pkg/parser"
	"github.com/yourusername/token-monitor/pkg/reader"
	"github.com/yourusername/token-monitor/pkg/session"
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
