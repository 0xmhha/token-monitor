package aggregator

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/0xmhha/token-monitor/pkg/parser"
)

// ModelBreakdown holds per-model token totals.
type ModelBreakdown struct {
	Model        string // exact model string from JSONL (e.g., "claude-sonnet-4-6")
	InputTokens  int
	OutputTokens int
	CacheCreate  int
	CacheRead    int
	TotalTokens  int
	EntryCount   int
}

// BreakdownByModel groups entries by exact model name.
// Skips entries with empty model or model == "<synthetic>".
func BreakdownByModel(entries []parser.UsageEntry) map[string]ModelBreakdown {
	out := make(map[string]ModelBreakdown)
	for _, e := range entries {
		m := e.Message.Model
		if m == "" || m == "<synthetic>" {
			continue
		}
		b := out[m]
		b.Model = m
		b.InputTokens += e.Message.Usage.InputTokens
		b.OutputTokens += e.Message.Usage.OutputTokens
		b.CacheCreate += e.Message.Usage.CacheCreationInputTokens
		b.CacheRead += e.Message.Usage.CacheReadInputTokens
		b.TotalTokens = b.InputTokens + b.OutputTokens + b.CacheCreate + b.CacheRead
		b.EntryCount++
		out[m] = b
	}
	return out
}

// MatchModel reports whether model matches glob (case-insensitive).
// Empty glob matches everything. Glob supports `*` wildcard via filepath.Match.
func MatchModel(model, glob string) bool {
	if glob == "" {
		return true
	}
	ok, err := filepath.Match(strings.ToLower(glob), strings.ToLower(model))
	if err != nil {
		return false
	}
	return ok
}

// FilterByModelGlob returns entries whose model matches the glob.
func FilterByModelGlob(entries []parser.UsageEntry, glob string) []parser.UsageEntry {
	if glob == "" {
		return entries
	}
	out := make([]parser.UsageEntry, 0, len(entries))
	for _, e := range entries {
		if MatchModel(e.Message.Model, glob) {
			out = append(out, e)
		}
	}
	return out
}

// FilterSince returns entries with Timestamp >= since.
func FilterSince(entries []parser.UsageEntry, since time.Time) []parser.UsageEntry {
	out := make([]parser.UsageEntry, 0, len(entries))
	for _, e := range entries {
		if !e.Timestamp.Before(since) {
			out = append(out, e)
		}
	}
	return out
}
