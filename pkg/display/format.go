package display

import (
	"fmt"
	"strings"
	"time"
)

// FormatCompact formats a number with K/M suffix for compact display.
//
// Rules:
//   - <1,000: exact number ("823")
//   - 1,000–999,999: K with 1 decimal ("12.5K")
//   - 1,000,000+: M with 1 decimal ("1.2M")
func FormatCompact(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000.0)
	case n >= 1000:
		return fmt.Sprintf("%.1fK", float64(n)/1000.0)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// FormatDuration formats a duration as a human-readable string
// (e.g. "3h42m", "45m", "12s", "0s").
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}

	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	return fmt.Sprintf("%ds", seconds)
}

// FormatRate formats a float rate value with 1 decimal place
// (e.g. "2145.3", "0.0").
func FormatRate(f float64) string {
	return fmt.Sprintf("%.1f", f)
}

// FormatTokenCount formats a token count with comma separators
// (e.g. "12,534", "1,234,567").
func FormatTokenCount(n int) string {
	if n < 0 {
		return "-" + formatPositiveWithCommas(-n)
	}
	return formatPositiveWithCommas(n)
}

// formatPositiveWithCommas inserts comma separators every 3 digits from the right
// for a non-negative integer.
func formatPositiveWithCommas(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var sb strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		sb.WriteString(s[:remainder])
	}

	for i := remainder; i < len(s); i += 3 {
		if sb.Len() > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(s[i : i+3])
	}

	return sb.String()
}

