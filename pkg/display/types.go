// Package display provides output formatting for token statistics.
//
// It supports multiple output formats (table, JSON, simple text)
// and handles statistics formatting for display.
package display

import (
	"io"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
)

// Format represents an output format.
type Format string

const (
	// FormatTable displays statistics in a formatted table.
	FormatTable Format = "table"

	// FormatJSON displays statistics as JSON.
	FormatJSON Format = "json"

	// FormatSimple displays statistics in simple text format.
	FormatSimple Format = "simple"
)

// Formatter formats and displays token statistics.
type Formatter interface {
	// FormatStats formats overall statistics.
	//
	// Parameters:
	//   - w: Output writer
	//   - stats: Statistics to format
	//
	// Returns error if formatting fails.
	FormatStats(w io.Writer, stats aggregator.Statistics) error

	// FormatGroupedStats formats grouped statistics.
	//
	// Parameters:
	//   - w: Output writer
	//   - grouped: Grouped statistics to format
	//   - dimensions: Dimension names for display
	//
	// Returns error if formatting fails.
	FormatGroupedStats(w io.Writer, grouped map[string]aggregator.Statistics, dimensions []string) error

	// FormatTopSessions formats top session statistics.
	//
	// Parameters:
	//   - w: Output writer
	//   - sessions: Session statistics to format
	//
	// Returns error if formatting fails.
	FormatTopSessions(w io.Writer, sessions []aggregator.SessionStats) error
}

// Config contains formatter configuration.
type Config struct {
	// Format specifies the output format.
	// Default: FormatTable.
	Format Format

	// ShowPercentiles enables percentile display.
	// Default: true.
	ShowPercentiles bool

	// ShowTimestamps enables timestamp display.
	// Default: true.
	ShowTimestamps bool

	// Compact enables compact output (less whitespace).
	// Default: false.
	Compact bool
}
