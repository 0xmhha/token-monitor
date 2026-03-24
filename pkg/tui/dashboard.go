package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
	"github.com/0xmhha/token-monitor/pkg/monitor"
)

// sessionDetail holds stats for a selected single session,
// split into past (before TUI start) and current (after TUI start).
type sessionDetail struct {
	sessionID   string
	filePath    string
	projectPath string

	// Three-way split
	pastStats    aggregator.Statistics // before TUI started
	currentStats aggregator.Statistics // after TUI started
	totalStats   aggregator.Statistics // all entries

	burnRate aggregator.BurnRate
	block    aggregator.BillingBlock
}

// dashboardView renders the real-time monitoring dashboard.
type dashboardView struct {
	lastUpdate *monitor.Update
	detail     *sessionDetail // non-nil when viewing a specific session
	width      int
	height     int
}

func newDashboardView() dashboardView {
	return dashboardView{}
}

func (d *dashboardView) setSize(width, height int) {
	d.width = width
	d.height = height
}

func (d *dashboardView) update(upd monitor.Update) {
	d.lastUpdate = &upd
}

func (d *dashboardView) setDetail(detail *sessionDetail) {
	d.detail = detail
}

func (d *dashboardView) clearDetail() {
	d.detail = nil
}

func (d *dashboardView) hasDetail() bool {
	return d.detail != nil
}

func (d *dashboardView) view() string {
	// Session detail mode
	if d.detail != nil {
		return d.viewSessionDetail()
	}

	// Live all-sessions mode
	if d.lastUpdate == nil {
		return lipgloss.Place(
			d.width, d.height,
			lipgloss.Center, lipgloss.Center,
			mutedStyle.Render("Waiting for data..."),
		)
	}

	upd := d.lastUpdate
	var sections []string

	// Header
	header := titleStyle.Render("Live Token Monitor") + "  " +
		mutedStyle.Render(upd.Timestamp.Format("2006-01-02 15:04:05"))
	sections = append(sections, header, "")

	// Token usage table
	sections = append(sections, d.tokenPanel(upd), "")

	// Stats + Burn rate side by side
	statsContent := d.statsPanel(upd.Stats)
	burnContent := d.burnRatePanel(upd.BurnRate)

	panelWidth := (d.width - 6) / 2
	if panelWidth < 30 {
		sections = append(sections, statsContent, "", burnContent)
	} else {
		left := lipgloss.NewStyle().Width(panelWidth).Render(statsContent)
		right := lipgloss.NewStyle().Width(panelWidth).Render(burnContent)
		sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
	}

	// Billing block
	if upd.CurrentBlock.EntryCount > 0 {
		sections = append(sections, "", d.billingPanel(upd.CurrentBlock))
	}

	// Timeline
	if !upd.Stats.FirstSeen.IsZero() {
		sections = append(sections, "", d.timeline(upd.Stats))
	}

	return strings.Join(sections, "\n")
}

func (d *dashboardView) tokenPanel(upd *monitor.Update) string {
	stats := upd.Stats
	delta := upd.Delta
	cum := upd.Cumulative

	title := panelTitleStyle.Render("Token Usage")

	// Dynamic column widths based on available panel width
	innerW := d.width - 10 // panel border + padding
	colLabel := max(16, innerW*30/100)
	colVal := max(12, (innerW-colLabel)/3)

	headerRow := "  " +
		cellLeft("Metric", colLabel, tableHeaderStyle) +
		cellRight("Total", colVal, tableHeaderStyle) +
		cellRight("Session +", colVal, tableHeaderStyle) +
		cellRight("Now +", colVal, tableHeaderStyle)

	mkRow := func(label string, total, session, now int) string {
		curStyle := mutedStyle
		if now > 0 {
			curStyle = successStyle
		}
		return "  " +
			cellLeft(label, colLabel, subtitleStyle) +
			cellRight(formatNum(total), colVal, valueStyle) +
			cellRight(formatDelta(session), colVal, accentStyle) +
			cellRight(formatDelta(now), colVal, curStyle)
	}

	rows := []string{
		mkRow("Requests", stats.Count, cum.NewEntries, delta.NewEntries),
		mkRow("Input Tokens", stats.InputTokens, cum.InputTokens, delta.InputTokens),
		mkRow("Output Tokens", stats.OutputTokens, cum.OutputTokens, delta.OutputTokens),
		mkRow("Total Tokens", stats.TotalTokens, cum.TotalTokens, delta.TotalTokens),
	}

	content := title + "\n" + headerRow + "\n" + strings.Join(rows, "\n")
	return panelStyle.Width(d.width - 4).Render(content)
}

func (d *dashboardView) statsPanel(stats aggregator.Statistics) string {
	title := panelTitleStyle.Render("Statistics")

	rows := []string{
		statRow("Average", fmt.Sprintf("%.0f", stats.AvgTokens)),
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

	content := title + "\n" + strings.Join(rows, "\n")
	return panelStyle.Render(content)
}

func (d *dashboardView) burnRatePanel(rate aggregator.BurnRate) string {
	title := panelTitleStyle.Render("Burn Rate (5min)")

	if rate.EntryCount == 0 {
		content := title + "\n" + mutedStyle.Render("  No recent activity")
		return panelStyle.Render(content)
	}

	rows := []string{
		statRow("Tokens/min", fmt.Sprintf("%.1f", rate.TokensPerMinute)),
		statRow("Tokens/hour", fmt.Sprintf("%.0f", rate.TokensPerHour)),
		statRow("Input/min", fmt.Sprintf("%.1f", rate.InputTokensPerMinute)),
		statRow("Output/min", fmt.Sprintf("%.1f", rate.OutputTokensPerMinute)),
		statRow("Entries", formatNum(rate.EntryCount)),
	}

	content := title + "\n" + strings.Join(rows, "\n")
	return panelStyle.Render(content)
}

func (d *dashboardView) billingPanel(block aggregator.BillingBlock) string {
	title := panelTitleStyle.Render(fmt.Sprintf("Billing Block (%s - %s UTC)",
		block.StartTime.UTC().Format("15:04"),
		block.EndTime.UTC().Format("15:04")))

	rows := []string{
		statRow("Total Tokens", formatNum(block.TotalTokens)),
		statRow("Input Tokens", formatNum(block.InputTokens)),
		statRow("Output Tokens", formatNum(block.OutputTokens)),
		statRow("Entries", formatNum(block.EntryCount)),
	}

	remaining := block.EndTime.Sub(time.Now().UTC())
	if remaining > 0 {
		hours := int(remaining.Hours())
		mins := int(remaining.Minutes()) % 60
		timeLeft := fmt.Sprintf("%dh%02dm", hours, mins)

		style := successStyle
		if remaining < 30*time.Minute {
			style = warningStyle
		}
		rows = append(rows, statRow("Time Left", style.Render(timeLeft)))
	}

	content := title + "\n" + strings.Join(rows, "\n")
	return highlightPanelStyle.Width(d.width - 4).Render(content)
}

func (d *dashboardView) timeline(stats aggregator.Statistics) string {
	duration := stats.LastSeen.Sub(stats.FirstSeen)
	durationStr := ""
	if duration > 0 {
		durationStr = " | Duration: " + duration.Round(time.Second).String()
	}

	return mutedStyle.Render(fmt.Sprintf("  First: %s | Last: %s%s",
		stats.FirstSeen.Format("15:04:05"),
		stats.LastSeen.Format("15:04:05"),
		durationStr,
	))
}

// viewSessionDetail renders stats for a single selected session
// with three-way split: Past / Current / Total.
func (d *dashboardView) viewSessionDetail() string {
	det := d.detail
	var sections []string

	// Header with session info — show full IDs, no truncation
	header := titleStyle.Render("Session Detail")
	sections = append(sections, header)

	sections = append(sections, accentStyle.Render("ID: "+det.sessionID))
	if det.projectPath != "" {
		sections = append(sections, mutedStyle.Render("Project: "+shortenProjectPath(det.projectPath)))
	}
	sections = append(sections, mutedStyle.Render("Press ESC to return to live view  |  R to refresh"))
	sections = append(sections, "")

	// Three-way token usage table
	sections = append(sections, d.tokenPanelThreeWay(det), "")

	// Stats (total) + Burn rate side by side
	statsContent := d.statsPanel(det.totalStats)
	burnContent := d.burnRatePanel(det.burnRate)

	panelWidth := (d.width - 6) / 2
	if panelWidth < 30 {
		sections = append(sections, statsContent, "", burnContent)
	} else {
		left := lipgloss.NewStyle().Width(panelWidth).Render(statsContent)
		right := lipgloss.NewStyle().Width(panelWidth).Render(burnContent)
		sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
	}

	// Billing block
	if det.block.EntryCount > 0 {
		sections = append(sections, "", d.billingPanel(det.block))
	}

	// Timeline
	if !det.totalStats.FirstSeen.IsZero() {
		sections = append(sections, "", d.timeline(det.totalStats))
	}

	return strings.Join(sections, "\n")
}

// tokenPanelThreeWay renders a three-column token table: Past / Current / Total.
func (d *dashboardView) tokenPanelThreeWay(det *sessionDetail) string {
	title := panelTitleStyle.Render("Token Usage")

	innerW := d.width - 10
	colLabel := max(16, innerW*30/100)
	colVal := max(12, (innerW-colLabel)/3)

	headerRow := "  " +
		cellLeft("Metric", colLabel, tableHeaderStyle) +
		cellRight("Past", colVal, tableHeaderStyle) +
		cellRight("Current", colVal, tableHeaderStyle) +
		cellRight("Total", colVal, tableHeaderStyle)

	past := det.pastStats
	cur := det.currentStats
	total := det.totalStats

	mkRow := func(label string, p, c, t int) string {
		curStyle := mutedStyle
		if c > 0 {
			curStyle = successStyle
		}
		return "  " +
			cellLeft(label, colLabel, subtitleStyle) +
			cellRight(formatNum(p), colVal, subtitleStyle) +
			cellRight(formatNum(c), colVal, curStyle) +
			cellRight(formatNum(t), colVal, valueStyle)
	}

	rows := []string{
		mkRow("Requests", past.Count, cur.Count, total.Count),
		mkRow("Input Tokens", past.InputTokens, cur.InputTokens, total.InputTokens),
		mkRow("Output Tokens", past.OutputTokens, cur.OutputTokens, total.OutputTokens),
		mkRow("Total Tokens", past.TotalTokens, cur.TotalTokens, total.TotalTokens),
	}

	content := title + "\n" + headerRow + "\n" + strings.Join(rows, "\n")
	return panelStyle.Width(d.width - 4).Render(content)
}

// Shared helpers

// cellLeft renders text left-aligned in a fixed-width cell with a style.
// lipgloss handles ANSI codes correctly, unlike fmt.Sprintf width specifiers.
func cellLeft(text string, width int, style lipgloss.Style) string {
	return style.Width(width).Align(lipgloss.Left).Render(text)
}

// cellRight renders text right-aligned in a fixed-width cell with a style.
func cellRight(text string, width int, style lipgloss.Style) string {
	return style.Width(width).Align(lipgloss.Right).Render(text)
}

func statRow(label, value string) string {
	return "  " + cellLeft(label, 18, subtitleStyle) + " " + valueStyle.Render(value)
}

func formatNum(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	b.Grow(len(s) + len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func formatDelta(n int) string {
	if n == 0 {
		return "+0"
	}
	return fmt.Sprintf("+%s", formatNum(n))
}

// shortenProjectPath strips the common Claude config prefix to show only
// the meaningful project identifier (e.g. "-Users-foo-Work-myapp").
func shortenProjectPath(path string) string {
	markers := []string{"/projects/", "/.claude/projects/", "/claude/projects/"}
	for _, marker := range markers {
		if idx := strings.LastIndex(path, marker); idx >= 0 {
			return path[idx+len(marker):]
		}
	}
	// Fallback: show last 2 path segments
	parts := strings.Split(path, "/")
	if len(parts) > 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return path
}
