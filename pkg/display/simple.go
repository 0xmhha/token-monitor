package display

import (
	"fmt"
	"io"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
)

// simpleFormatter formats output as simple text.
type simpleFormatter struct {
	config Config
}

// FormatStats implements Formatter.FormatStats.
func (f *simpleFormatter) FormatStats(w io.Writer, stats aggregator.Statistics) error {
	_, err := fmt.Fprintf(w, "Entries: %d | Sessions: %d | Total: %s | Avg: %s | Min: %s | Max: %s\n",
		stats.Count,
		stats.SessionCount,
		formatNumber(stats.TotalTokens),
		formatFloat(stats.AvgTokens, 1),
		formatNumber(stats.MinTokens),
		formatNumber(stats.MaxTokens))
	return err
}

// FormatGroupedStats implements Formatter.FormatGroupedStats.
func (f *simpleFormatter) FormatGroupedStats(w io.Writer, grouped map[string]aggregator.Statistics, dimensions []string) error {
	if err := validateDimensions(dimensions); err != nil {
		return err
	}

	for key, stats := range grouped {
		if _, err := fmt.Fprintf(w, "%s: %d entries, %s tokens (avg: %s)\n",
			key,
			stats.Count,
			formatNumber(stats.TotalTokens),
			formatFloat(stats.AvgTokens, 1)); err != nil {
			return err
		}
	}

	return nil
}

// FormatTopSessions implements Formatter.FormatTopSessions.
func (f *simpleFormatter) FormatTopSessions(w io.Writer, sessions []aggregator.SessionStats) error {
	for i, session := range sessions {
		if _, err := fmt.Fprintf(w, "#%d: %s (%s) - %s tokens in %d entries\n",
			i+1,
			session.SessionID,
			session.Model,
			formatNumber(session.Statistics.TotalTokens),
			session.Statistics.Count); err != nil {
			return err
		}
	}

	return nil
}
