// Package display provides formatting helpers for the token-monitor CLI.
//
// ParseWindow lives here (rather than in pkg/aggregator) because the
// "window string" is a user-facing input format — it sits at the same
// level of abstraction as other display helpers like FormatCompact and
// FormatDuration.
package display

import (
	"fmt"
	"strings"
	"time"
)

// ParseWindow converts a user-facing window string to a "since" cutoff time.
//
// Supported inputs (case-insensitive):
//   - "" or "today": midnight in now.Location()
//   - "all":         zero time.Time{} (callers should treat this as "no cutoff")
//   - "Nd":          now - N*24h (N must be a positive integer)
//   - "Nh":          now - N hours
//
// Any other input — including "7" without a unit, "abcd", or "7x" —
// returns a non-nil error.
//
// Note on the zero return value: ParseWindow("all") returns (time.Time{}, nil).
// Callers using FilterSince should pass that zero value through unchanged;
// FilterSince already documents that a zero cutoff includes all entries.
func ParseWindow(s string, now time.Time) (time.Time, error) {
	normalized := strings.ToLower(strings.TrimSpace(s))
	switch normalized {
	case "", "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	case "all":
		return time.Time{}, nil
	}

	if t, ok := parseDurationSuffix(normalized, "d", 24*time.Hour, now); ok {
		return t, nil
	}
	if t, ok := parseDurationSuffix(normalized, "h", time.Hour, now); ok {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid window: %q (expected today, all, Nd, or Nh)", s)
}

// parseDurationSuffix tries to parse a string like "7d" or "24h" by stripping
// the suffix, parsing the rest as a positive integer, and subtracting that
// many `unit` durations from `now`.
//
// Returns ok=false if the string does not have the suffix, has no leading
// digits, or contains anything other than digits before the suffix. This
// rejects malformed inputs like "abcd", "7", "-1d", "1.5h", and "7x".
func parseDurationSuffix(s, suffix string, unit time.Duration, now time.Time) (time.Time, bool) {
	if !strings.HasSuffix(s, suffix) {
		return time.Time{}, false
	}
	num := strings.TrimSuffix(s, suffix)
	if num == "" {
		return time.Time{}, false
	}
	// Strict integer parse: only ASCII digits allowed.
	// Using fmt.Sscanf would silently accept "7x" as 7; require exact match.
	for _, r := range num {
		if r < '0' || r > '9' {
			return time.Time{}, false
		}
	}
	var n int
	if _, err := fmt.Sscanf(num, "%d", &n); err != nil {
		return time.Time{}, false
	}
	if n <= 0 {
		return time.Time{}, false
	}
	return now.Add(-time.Duration(n) * unit), true
}
