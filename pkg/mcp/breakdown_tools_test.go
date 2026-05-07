package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xmhha/token-monitor/pkg/discovery"
)

// makeMultiModelSession writes a JSONL fixture containing one entry per
// (model, tokens) tuple. All entries use the same sessionID and a recent
// timestamp so they pass through "today" filtering. Returns a SessionFile
// suitable for handing to a mockDiscoverer.
//
// `entries` is a slice of (model, input, output) tuples. The synthetic
// model literal "<synthetic>" is supported and should be filtered out by
// the breakdown aggregator.
type modelEntry struct {
	model           string
	input           int
	output          int
	cacheCreate     int
	cacheRead       int
	timestampOffset time.Duration // negative = past
}

func makeMultiModelSession(t *testing.T, sessionID string, entries []modelEntry) discovery.SessionFile {
	t.Helper()

	dir := t.TempDir()
	filePath := filepath.Join(dir, sessionID+".jsonl")

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck

	now := time.Now().UTC()
	for i, e := range entries {
		ts := now.Add(e.timestampOffset)
		entry := map[string]any{
			"timestamp": ts.Format(time.RFC3339Nano),
			"sessionId": sessionID,
			"version":   "1",
			"cwd":       dir,
			"message": map[string]any{
				"id":    fmt.Sprintf("msg_%d", i),
				"model": e.model,
				"usage": map[string]any{
					"input_tokens":                e.input,
					"output_tokens":               e.output,
					"cache_creation_input_tokens": e.cacheCreate,
					"cache_read_input_tokens":     e.cacheRead,
				},
				"content": []any{},
			},
		}
		data, marshalErr := json.Marshal(entry)
		require.NoError(t, marshalErr)
		_, writeErr := f.Write(append(data, '\n'))
		require.NoError(t, writeErr)
	}

	require.NoError(t, f.Close())

	info, err := os.Stat(filePath)
	require.NoError(t, err)

	return discovery.SessionFile{
		SessionID:   sessionID,
		FilePath:    filePath,
		ProjectPath: dir,
		Size:        info.Size(),
		ModTime:     info.ModTime().Unix(),
	}
}

// --- get_session_breakdown ---

func TestGetSessionBreakdown_MultipleModels(t *testing.T) {
	t.Parallel()

	const sessionID = "11111111-1111-1111-1111-111111111111"
	sf := makeMultiModelSession(t, sessionID, []modelEntry{
		{model: "claude-sonnet-4-6", input: 100, output: 50, timestampOffset: -10 * time.Minute},
		{model: "claude-opus-4-7", input: 500, output: 200, timestampOffset: -5 * time.Minute},
		{model: "claude-sonnet-4-6", input: 30, output: 20, timestampOffset: -3 * time.Minute},
	})

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sf}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	result, err := registry.Call("get_session_breakdown", json.RawMessage(`{}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))
	assert.Equal(t, sessionID, data["session_id"])

	breakdown, ok := data["breakdown"].([]any)
	require.True(t, ok, "breakdown must be an array")
	require.Len(t, breakdown, 2, "expected one entry per distinct model")

	// Sorted by total_tokens desc: opus (700) > sonnet (200).
	first, _ := breakdown[0].(map[string]any)
	second, _ := breakdown[1].(map[string]any)
	assert.Equal(t, "claude-opus-4-7", first["model"])
	assert.EqualValues(t, 700, first["total_tokens"])
	assert.EqualValues(t, 1, first["entry_count"])
	assert.Equal(t, "claude-sonnet-4-6", second["model"])
	assert.EqualValues(t, 200, second["total_tokens"])
	assert.EqualValues(t, 2, second["entry_count"])
}

func TestGetSessionBreakdown_SkipsSyntheticModel(t *testing.T) {
	t.Parallel()

	const sessionID = "22222222-2222-2222-2222-222222222222"
	sf := makeMultiModelSession(t, sessionID, []modelEntry{
		{model: "<synthetic>", input: 999, output: 999, timestampOffset: -1 * time.Minute},
		{model: "claude-sonnet-4-6", input: 10, output: 5, timestampOffset: -1 * time.Minute},
	})

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sf}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	result, err := registry.Call("get_session_breakdown", json.RawMessage(`{}`))
	require.NoError(t, err)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))
	breakdown, ok := data["breakdown"].([]any)
	require.True(t, ok)
	require.Len(t, breakdown, 1, "synthetic model entry must be filtered out")
	first, _ := breakdown[0].(map[string]any)
	assert.Equal(t, "claude-sonnet-4-6", first["model"])
}

func TestGetSessionBreakdown_NoCurrentSession(t *testing.T) {
	t.Parallel()

	disc := &mockDiscoverer{sessions: nil}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	_, err := registry.Call("get_session_breakdown", json.RawMessage(`{}`))
	require.Error(t, err, "expected error when no session is available")
}

// --- get_today_usage ---

func TestGetTodayUsage_AggregatesAcrossSessions(t *testing.T) {
	t.Parallel()

	// Offsets in seconds, not hours — keeps entries inside the "today"
	// window across timezones (CI runs UTC; nearby midnight could push
	// hour-offsets to "yesterday").
	sfA := makeMultiModelSession(t, "aaaaaaaa-1111-2222-3333-444444444444", []modelEntry{
		{model: "claude-sonnet-4-6", input: 100, output: 50, timestampOffset: -3 * time.Second},
	})
	sfB := makeMultiModelSession(t, "bbbbbbbb-1111-2222-3333-444444444444", []modelEntry{
		{model: "claude-opus-4-7", input: 200, output: 80, timestampOffset: -5 * time.Second},
		{model: "claude-sonnet-4-6", input: 50, output: 25, timestampOffset: -1 * time.Second},
	})

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sfA, sfB}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	result, err := registry.Call("get_today_usage", json.RawMessage(`{}`))
	require.NoError(t, err)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))

	assert.Equal(t, "today", data["window"])
	assert.NotEmpty(t, data["since"])

	// 100+50 + 200+80 + 50+25 = 505
	assert.EqualValues(t, 505, data["total_tokens"])
	assert.EqualValues(t, 2, data["session_count"])

	byModel, ok := data["by_model"].([]any)
	require.True(t, ok)
	require.Len(t, byModel, 2, "two distinct models across both sessions")
	first, _ := byModel[0].(map[string]any)
	// Opus (280) > Sonnet (225)
	assert.Equal(t, "claude-opus-4-7", first["model"])
	assert.EqualValues(t, 280, first["total_tokens"])
}

func TestGetTodayUsage_GlobFiltersByModel(t *testing.T) {
	t.Parallel()

	// Offsets in seconds, not hours — keeps entries inside the "today"
	// window across timezones (CI runs UTC; nearby midnight could push
	// hour-offsets to "yesterday").
	sf := makeMultiModelSession(t, "cccccccc-1111-2222-3333-444444444444", []modelEntry{
		{model: "claude-sonnet-4-6", input: 100, output: 50, timestampOffset: -2 * time.Second},
		{model: "claude-opus-4-7", input: 500, output: 200, timestampOffset: -4 * time.Second},
	})

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sf}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	result, err := registry.Call("get_today_usage", json.RawMessage(`{"model_glob":"*sonnet*"}`))
	require.NoError(t, err)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))

	assert.EqualValues(t, 150, data["total_tokens"], "only sonnet entries should be summed")
	byModel, ok := data["by_model"].([]any)
	require.True(t, ok)
	require.Len(t, byModel, 1)
	first, _ := byModel[0].(map[string]any)
	assert.Equal(t, "claude-sonnet-4-6", first["model"])
}

func TestGetTodayUsage_EmptyWhenNoSessions(t *testing.T) {
	t.Parallel()

	disc := &mockDiscoverer{sessions: nil}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	result, err := registry.Call("get_today_usage", json.RawMessage(`{}`))
	require.NoError(t, err)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))

	assert.Equal(t, "today", data["window"])
	assert.EqualValues(t, 0, data["total_tokens"])
	assert.EqualValues(t, 0, data["session_count"])
	byModel, ok := data["by_model"].([]any)
	require.True(t, ok)
	assert.Len(t, byModel, 0, "by_model must be an empty array, not nil")
}

// --- get_usage_by_window ---

func TestGetUsageByWindow_AcceptsValidWindow(t *testing.T) {
	t.Parallel()

	// 6h ago should be inside a 7d window but outside a 1h window.
	sf := makeMultiModelSession(t, "dddddddd-1111-2222-3333-444444444444", []modelEntry{
		{model: "claude-sonnet-4-6", input: 200, output: 100, timestampOffset: -6 * time.Hour},
	})

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sf}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	// 7d window: should include the entry.
	result, err := registry.Call("get_usage_by_window", json.RawMessage(`{"window":"7d"}`))
	require.NoError(t, err)
	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))
	assert.Equal(t, "7d", data["window"])
	assert.EqualValues(t, 300, data["total_tokens"])

	// 1h window: should exclude the 6h-old entry.
	result, err = registry.Call("get_usage_by_window", json.RawMessage(`{"window":"1h"}`))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))
	assert.EqualValues(t, 0, data["total_tokens"])
}

func TestGetUsageByWindow_AllWindowEmptySince(t *testing.T) {
	t.Parallel()

	sf := makeMultiModelSession(t, "eeeeeeee-1111-2222-3333-444444444444", []modelEntry{
		// Use a far-past timestamp to confirm "all" doesn't filter it out.
		{model: "claude-sonnet-4-6", input: 10, output: 5, timestampOffset: -90 * 24 * time.Hour},
	})

	disc := &mockDiscoverer{sessions: []discovery.SessionFile{sf}}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	result, err := registry.Call("get_usage_by_window", json.RawMessage(`{"window":"all"}`))
	require.NoError(t, err)

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &data))
	assert.Equal(t, "all", data["window"])
	// "all" maps to a zero time.Time; our handler reports that as "".
	assert.Equal(t, "", data["since"])
	assert.EqualValues(t, 15, data["total_tokens"], "the 90-day-old entry must be included")
}

func TestGetUsageByWindow_RequiresWindow(t *testing.T) {
	t.Parallel()

	disc := &mockDiscoverer{}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	_, err := registry.Call("get_usage_by_window", json.RawMessage(`{}`))
	require.Error(t, err, "window is a required parameter")
}

func TestGetUsageByWindow_InvalidWindowReturnsError(t *testing.T) {
	t.Parallel()

	disc := &mockDiscoverer{}
	registry := NewToolRegistry()
	RegisterTokenTools(registry, disc, newTestReaderFactory(), &testLogger{})

	_, err := registry.Call("get_usage_by_window", json.RawMessage(`{"window":"not-a-window"}`))
	require.Error(t, err, "invalid window format must produce an error")
	assert.Contains(t, err.Error(), "invalid window")
}
