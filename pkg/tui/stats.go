package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
)

// statsView renders aggregated statistics.
type statsView struct {
	stats       *aggregator.Statistics
	topSessions []aggregator.SessionStats
	width       int
	height      int
}

func newStatsView() statsView {
	return statsView{}
}

func (v *statsView) setSize(width, height int) {
	v.width = width
	v.height = height
}

func (v *statsView) setStats(stats aggregator.Statistics) {
	v.stats = &stats
}

func (v *statsView) setTopSessions(sessions []aggregator.SessionStats) {
	v.topSessions = sessions
}

func (v *statsView) view() string {
	if v.stats == nil {
		return lipgloss.Place(
			v.width, v.height,
			lipgloss.Center, lipgloss.Center,
			mutedStyle.Render("No statistics available"),
		)
	}

	var sections []string

	title := titleStyle.Render("Token Usage Statistics")
	sections = append(sections, title, "")

	// Overview panel
	sections = append(sections, v.overviewPanel())
	sections = append(sections, "")

	// Top sessions
	if len(v.topSessions) > 0 {
		sections = append(sections, v.topSessionsPanel())
	}

	return strings.Join(sections, "\n")
}

func (v *statsView) overviewPanel() string {
	stats := v.stats

	title := panelTitleStyle.Render("Overview")
	rows := []string{
		statRow("Entries", formatNum(stats.Count)),
		statRow("Sessions", formatNum(stats.SessionCount)),
		statRow("Total Tokens", formatNum(stats.TotalTokens)),
		statRow("Input Tokens", formatNum(stats.InputTokens)),
		statRow("Output Tokens", formatNum(stats.OutputTokens)),
		statRow("Average", fmt.Sprintf("%.2f", stats.AvgTokens)),
		statRow("Min", formatNum(stats.MinTokens)),
		statRow("Max", formatNum(stats.MaxTokens)),
	}

	if stats.P50Tokens > 0 {
		rows = append(rows,
			statRow("P50", formatNum(stats.P50Tokens)),
			statRow("P95", formatNum(stats.P95Tokens)),
			statRow("P99", formatNum(stats.P99Tokens)),
		)
	}

	if !stats.FirstSeen.IsZero() {
		rows = append(rows,
			statRow("First Seen", stats.FirstSeen.Format("2006-01-02 15:04:05")),
			statRow("Last Seen", stats.LastSeen.Format("2006-01-02 15:04:05")),
		)
	}

	content := title + "\n" + strings.Join(rows, "\n")
	return panelStyle.Width(v.width - 4).Render(content)
}

func (v *statsView) topSessionsPanel() string {
	title := panelTitleStyle.Render("Top Sessions")

	header := fmt.Sprintf("  %-4s %-20s %-16s %12s %12s",
		tableHeaderStyle.Render("#"),
		tableHeaderStyle.Render("Session"),
		tableHeaderStyle.Render("Model"),
		tableHeaderStyle.Render("Total"),
		tableHeaderStyle.Render("Avg"),
	)

	rows := make([]string, 0, len(v.topSessions)+1)
	rows = append(rows, header)

	for i, sess := range v.topSessions {
		row := fmt.Sprintf("  %-4s %-20s %-16s %12s %12s",
			accentStyle.Render(fmt.Sprintf("#%d", i+1)),
			truncate(sess.SessionID, 18),
			truncate(sess.Model, 14),
			valueStyle.Render(formatNum(sess.Statistics.TotalTokens)),
			mutedStyle.Render(fmt.Sprintf("%.0f", sess.Statistics.AvgTokens)),
		)
		rows = append(rows, row)
	}

	content := title + "\n" + strings.Join(rows, "\n")
	return panelStyle.Width(v.width - 4).Render(content)
}
