package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
	"github.com/0xmhha/token-monitor/pkg/discovery"
	"github.com/0xmhha/token-monitor/pkg/display"
	"github.com/0xmhha/token-monitor/pkg/parser"
	"github.com/0xmhha/token-monitor/pkg/reader"
)

// loadAllEntries discovers every session under the configured Claude config
// dirs and returns the merged stream of usage entries plus the underlying
// SessionFile list. Read errors on individual sessions are logged via `log`
// and skipped (matching status_command.go's tolerance) so a single corrupt
// JSONL file doesn't poison cross-session aggregation.
//
// This helper exists to deduplicate the reader+discover plumbing between
// the new breakdown/window MCP tools and to mirror the behavior of
// status_command.go's collect/collectEntries pair.
func loadAllEntries(
	disc discovery.Discoverer,
	readerFactory func() (reader.Reader, error),
	log Logger,
) ([]parser.UsageEntry, []discovery.SessionFile, error) {
	sessions, err := disc.Discover()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to discover sessions: %w", err)
	}

	r, err := readerFactory()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer r.Close() //nolint:errcheck

	ctx := context.Background()
	all := make([]parser.UsageEntry, 0, 1024)
	for _, sess := range sessions {
		entries, _, readErr := r.ReadFrom(ctx, sess.FilePath, 0)
		if readErr != nil {
			log.Warn("failed to read session", "session", sess.SessionID, "error", readErr)
			continue
		}
		all = append(all, entries...)
	}
	return all, sessions, nil
}

// --- Tool definitions ---

func toolGetSessionBreakdown() ToolDefinition {
	return ToolDefinition{
		Name:        "get_session_breakdown",
		Description: "Get token usage broken down by model for the current or specified session. Synthetic-model entries are excluded.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"session_id": map[string]any{"type": "string", "description": "Session ID. If omitted, uses current session."},
			},
		},
	}
}

func toolGetTodayUsage() ToolDefinition {
	return ToolDefinition{
		Name:        "get_today_usage",
		Description: "Get cumulative token usage today across all sessions, optionally filtered by model glob. Synthetic-model entries are excluded.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"model_glob": map[string]any{"type": "string", "description": "Glob pattern like '*sonnet*'. Empty = all models."},
			},
		},
	}
}

func toolGetUsageByWindow() ToolDefinition {
	return ToolDefinition{
		Name:        "get_usage_by_window",
		Description: "Get token usage for an arbitrary time window with optional model filter. Synthetic-model entries are excluded.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"window"},
			"properties": map[string]any{
				"window":     map[string]any{"type": "string", "description": "Window: today, all, Nd (e.g. 7d), or Nh (e.g. 24h)."},
				"model_glob": map[string]any{"type": "string", "description": "Glob pattern like '*sonnet*'. Empty = all models."},
			},
		},
	}
}

// --- Helpers ---

// breakdownToList converts a model→breakdown map into a deterministic
// JSON-friendly slice sorted by total_tokens desc, with model name as
// the tiebreaker so identical-total entries don't flip between calls.
func breakdownToList(m map[string]aggregator.ModelBreakdown) []map[string]any {
	out := make([]map[string]any, 0, len(m))
	for _, b := range m {
		out = append(out, map[string]any{
			"model":         b.Model,
			"total_tokens":  b.TotalTokens,
			"input_tokens":  b.InputTokens,
			"output_tokens": b.OutputTokens,
			"cache_create":  b.CacheCreate,
			"cache_read":    b.CacheRead,
			"entry_count":   b.EntryCount,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		ti, _ := out[i]["total_tokens"].(int)
		tj, _ := out[j]["total_tokens"].(int)
		if ti != tj {
			return ti > tj
		}
		mi, _ := out[i]["model"].(string)
		mj, _ := out[j]["model"].(string)
		return mi < mj
	})
	return out
}

// sumBreakdown returns the total tokens across every model in the breakdown.
func sumBreakdown(m map[string]aggregator.ModelBreakdown) int {
	total := 0
	for _, b := range m {
		total += b.TotalTokens
	}
	return total
}

// countContributingSessions returns the number of distinct sessions that
// contributed at least one entry to `entries` (post-filter). Only entries
// with non-empty SessionID count.
func countContributingSessions(entries []parser.UsageEntry) int {
	seen := make(map[string]struct{})
	for _, e := range entries {
		if e.SessionID == "" {
			continue
		}
		seen[e.SessionID] = struct{}{}
	}
	return len(seen)
}

// formatSince renders a since cutoff for the JSON response. The zero
// time.Time{} (returned by ParseWindow("all")) is reported as the empty
// string so callers can tell "no cutoff" apart from a real timestamp.
func formatSince(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// --- Tool handlers ---

func (c *sessionContext) handleGetSessionBreakdown(args json.RawMessage) (*ToolCallResult, error) {
	var params struct {
		SessionID string `json:"session_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	sf, err := c.resolveSession(params.SessionID)
	if err != nil {
		return nil, err
	}

	r, err := c.readerFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer r.Close() //nolint:errcheck

	entries, _, err := r.ReadFrom(context.Background(), sf.FilePath, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	breakdown := aggregator.BreakdownByModel(entries)
	return textResult(map[string]any{
		"session_id": sf.SessionID,
		"breakdown":  breakdownToList(breakdown),
	})
}

func (c *sessionContext) handleGetTodayUsage(args json.RawMessage) (*ToolCallResult, error) {
	var params struct {
		ModelGlob string `json:"model_glob"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	return c.usageByWindow("today", params.ModelGlob)
}

func (c *sessionContext) handleGetUsageByWindow(args json.RawMessage) (*ToolCallResult, error) {
	var params struct {
		Window    string `json:"window"`
		ModelGlob string `json:"model_glob"`
	}
	// Mirror sibling handlers: tolerate empty/null arguments so the
	// "window required" message surfaces instead of a JSON parse error
	// when a caller forgets the params block entirely.
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}
	if params.Window == "" {
		return nil, NewParamError("window is required")
	}

	return c.usageByWindow(params.Window, params.ModelGlob)
}

// usageByWindow is the shared implementation for get_today_usage and
// get_usage_by_window. It loads all sessions, applies window+glob filters,
// and emits the canonical {window, since, total_tokens, session_count, by_model}
// JSON shape.
func (c *sessionContext) usageByWindow(window, modelGlob string) (*ToolCallResult, error) {
	// display.ParseWindow already returns "invalid window: <q> (expected ...)"
	// — propagating it as ParamError without an extra prefix avoids the
	// double "invalid window: invalid window:" stutter.
	since, err := display.ParseWindow(window, time.Now())
	if err != nil {
		return nil, NewParamError(err.Error())
	}

	entries, _, err := loadAllEntries(c.disc, c.readerFactory, c.log)
	if err != nil {
		return nil, err
	}

	filtered := aggregator.FilterSince(entries, since)
	filtered = aggregator.FilterByModelGlob(filtered, modelGlob)
	breakdown := aggregator.BreakdownByModel(filtered)

	return textResult(map[string]any{
		"window":        window,
		"since":         formatSince(since),
		"total_tokens":  sumBreakdown(breakdown),
		"session_count": countContributingSessions(filtered),
		"by_model":      breakdownToList(breakdown),
	})
}
