package display

import (
	"fmt"
	"io"
	"strings"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
)

// tableFormatter formats output as tables.
type tableFormatter struct {
	config Config
}

// FormatStats implements Formatter.FormatStats.
func (f *tableFormatter) FormatStats(w io.Writer, stats aggregator.Statistics) error {
	if err := writeHeader(w, "Token Usage Statistics", f.config.Compact); err != nil {
		return err
	}

	rows := [][]string{
		{"Entries", formatNumber(stats.Count)},
		{"Sessions", formatNumber(stats.SessionCount)},
		{"Total Tokens", formatNumber(stats.TotalTokens)},
		{"Input Tokens", formatNumber(stats.InputTokens)},
		{"Output Tokens", formatNumber(stats.OutputTokens)},
		{"Average Tokens", formatFloat(stats.AvgTokens, 2)},
		{"Min Tokens", formatNumber(stats.MinTokens)},
		{"Max Tokens", formatNumber(stats.MaxTokens)},
	}

	if f.config.ShowPercentiles {
		rows = append(rows,
			[]string{"P50 Tokens", formatNumber(stats.P50Tokens)},
			[]string{"P95 Tokens", formatNumber(stats.P95Tokens)},
			[]string{"P99 Tokens", formatNumber(stats.P99Tokens)},
		)
	}

	if f.config.ShowTimestamps && !stats.FirstSeen.IsZero() {
		rows = append(rows,
			[]string{"First Seen", stats.FirstSeen.Format("2006-01-02 15:04:05")},
			[]string{"Last Seen", stats.LastSeen.Format("2006-01-02 15:04:05")},
		)
	}

	return f.writeTable(w, []string{"Metric", "Value"}, rows)
}

// FormatGroupedStats implements Formatter.FormatGroupedStats.
func (f *tableFormatter) FormatGroupedStats(w io.Writer, grouped map[string]aggregator.Statistics, dimensions []string) error {
	if err := validateDimensions(dimensions); err != nil {
		return err
	}

	if err := writeHeader(w, "Grouped Statistics", f.config.Compact); err != nil {
		return err
	}

	// Build header.
	header := make([]string, len(dimensions)+6)
	copy(header, dimensions)
	header[len(dimensions)] = "Entries"
	header[len(dimensions)+1] = "Total"
	header[len(dimensions)+2] = "Input"
	header[len(dimensions)+3] = "Output"
	header[len(dimensions)+4] = "Avg"
	header[len(dimensions)+5] = "Min/Max"

	// Build rows.
	rows := make([][]string, 0, len(grouped))
	for key, stats := range grouped {
		row := make([]string, len(header))

		// Parse key into dimension values.
		parts := strings.Split(key, "|")
		for i, part := range parts {
			if i < len(dimensions) {
				row[i] = part
			}
		}

		// Add statistics.
		row[len(dimensions)] = formatNumber(stats.Count)
		row[len(dimensions)+1] = formatNumber(stats.TotalTokens)
		row[len(dimensions)+2] = formatNumber(stats.InputTokens)
		row[len(dimensions)+3] = formatNumber(stats.OutputTokens)
		row[len(dimensions)+4] = formatFloat(stats.AvgTokens, 1)
		row[len(dimensions)+5] = fmt.Sprintf("%s/%s",
			formatNumber(stats.MinTokens),
			formatNumber(stats.MaxTokens))

		rows = append(rows, row)
	}

	return f.writeTable(w, header, rows)
}

// FormatTopSessions implements Formatter.FormatTopSessions.
func (f *tableFormatter) FormatTopSessions(w io.Writer, sessions []aggregator.SessionStats) error {
	if err := writeHeader(w, "Top Sessions by Token Usage", f.config.Compact); err != nil {
		return err
	}

	header := []string{"Rank", "Session ID", "Model", "Entries", "Total Tokens", "Input", "Output", "Avg"}

	rows := make([][]string, len(sessions))
	for i, session := range sessions {
		rows[i] = []string{
			fmt.Sprintf("#%d", i+1),
			session.SessionID,
			session.Model,
			formatNumber(session.Statistics.Count),
			formatNumber(session.Statistics.TotalTokens),
			formatNumber(session.Statistics.InputTokens),
			formatNumber(session.Statistics.OutputTokens),
			formatFloat(session.Statistics.AvgTokens, 1),
		}
	}

	return f.writeTable(w, header, rows)
}

// writeTable writes a formatted table.
func (f *tableFormatter) writeTable(w io.Writer, header []string, rows [][]string) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "No data")
		return err
	}

	// Calculate column widths.
	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = len(h)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Write header.
	if err := f.writeRow(w, header, widths); err != nil {
		return err
	}

	// Write separator.
	if !f.config.Compact {
		separator := make([]string, len(header))
		for i, width := range widths {
			separator[i] = strings.Repeat("-", width)
		}
		if err := f.writeRow(w, separator, widths); err != nil {
			return err
		}
	}

	// Write rows.
	for _, row := range rows {
		if err := f.writeRow(w, row, widths); err != nil {
			return err
		}
	}

	// Add spacing.
	if !f.config.Compact {
		_, err := fmt.Fprintln(w)
		return err
	}

	return nil
}

// writeRow writes a single table row.
func (f *tableFormatter) writeRow(w io.Writer, cells []string, widths []int) error {
	for i, cell := range cells {
		if i > 0 {
			if f.config.Compact {
				if _, err := fmt.Fprint(w, " "); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprint(w, "  "); err != nil {
					return err
				}
			}
		}

		format := fmt.Sprintf("%%-%ds", widths[i])
		if _, err := fmt.Fprintf(w, format, cell); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintln(w)
	return err
}
