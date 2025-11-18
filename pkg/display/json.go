package display

import (
	"encoding/json"
	"io"

	"github.com/yourusername/token-monitor/pkg/aggregator"
)

// jsonFormatter formats output as JSON.
type jsonFormatter struct {
	config Config
}

// FormatStats implements Formatter.FormatStats.
func (f *jsonFormatter) FormatStats(w io.Writer, stats aggregator.Statistics) error {
	encoder := json.NewEncoder(w)
	if !f.config.Compact {
		encoder.SetIndent("", "  ")
	}

	return encoder.Encode(stats)
}

// FormatGroupedStats implements Formatter.FormatGroupedStats.
func (f *jsonFormatter) FormatGroupedStats(w io.Writer, grouped map[string]aggregator.Statistics, dimensions []string) error {
	if err := validateDimensions(dimensions); err != nil {
		return err
	}

	encoder := json.NewEncoder(w)
	if !f.config.Compact {
		encoder.SetIndent("", "  ")
	}

	return encoder.Encode(grouped)
}

// FormatTopSessions implements Formatter.FormatTopSessions.
func (f *jsonFormatter) FormatTopSessions(w io.Writer, sessions []aggregator.SessionStats) error {
	encoder := json.NewEncoder(w)
	if !f.config.Compact {
		encoder.SetIndent("", "  ")
	}

	return encoder.Encode(sessions)
}
