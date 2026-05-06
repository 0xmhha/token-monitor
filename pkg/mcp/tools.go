package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
	"github.com/0xmhha/token-monitor/pkg/discovery"
	"github.com/0xmhha/token-monitor/pkg/reader"
)

// ToolHandler is a function that handles a tool call.
type ToolHandler func(args json.RawMessage) (*ToolCallResult, error)

// ToolRegistry holds tool definitions and their handlers.
type ToolRegistry struct {
	tools    []ToolDefinition
	handlers map[string]ToolHandler
}

// NewToolRegistry creates an empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:    make([]ToolDefinition, 0),
		handlers: make(map[string]ToolHandler),
	}
}

// Register adds a tool definition and its handler to the registry.
func (r *ToolRegistry) Register(def ToolDefinition, handler ToolHandler) {
	r.tools = append(r.tools, def)
	r.handlers[def.Name] = handler
}

// List returns all registered tool definitions.
func (r *ToolRegistry) List() []ToolDefinition {
	result := make([]ToolDefinition, len(r.tools))
	copy(result, r.tools)
	return result
}

// Call invokes the handler for the named tool.
func (r *ToolRegistry) Call(name string, args json.RawMessage) (*ToolCallResult, error) {
	handler, ok := r.handlers[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	return handler(args)
}

// sessionContext holds shared dependencies used by all tool handlers.
type sessionContext struct {
	disc          discovery.Discoverer
	readerFactory func() (reader.Reader, error)
	log           Logger
}

// RegisterTokenTools registers all token monitoring tools into the registry.
//
// Currently registers 9 tools, split into two groups:
//   - Single-session: get_token_usage, get_burn_rate, get_billing_block,
//     list_sessions, get_session_detail, compare_sessions
//   - Cross-session breakdown (v0.2): get_session_breakdown,
//     get_today_usage, get_usage_by_window
func RegisterTokenTools(registry *ToolRegistry, disc discovery.Discoverer, readerFactory func() (reader.Reader, error), log Logger) {
	ctx := &sessionContext{disc: disc, readerFactory: readerFactory, log: log}

	registry.Register(toolGetTokenUsage(), ctx.handleGetTokenUsage)
	registry.Register(toolGetBurnRate(), ctx.handleGetBurnRate)
	registry.Register(toolGetBillingBlock(), ctx.handleGetBillingBlock)
	registry.Register(toolListSessions(), ctx.handleListSessions)
	registry.Register(toolGetSessionDetail(), ctx.handleGetSessionDetail)
	registry.Register(toolCompareSessions(), ctx.handleCompareSessions)

	// v0.2 cross-session breakdown tools.
	registry.Register(toolGetSessionBreakdown(), ctx.handleGetSessionBreakdown)
	registry.Register(toolGetTodayUsage(), ctx.handleGetTodayUsage)
	registry.Register(toolGetUsageByWindow(), ctx.handleGetUsageByWindow)
}

// --- Tool definitions ---

func toolGetTokenUsage() ToolDefinition {
	return ToolDefinition{
		Name:        "get_token_usage",
		Description: "Get token usage statistics for a session. Auto-detects the current session if session_id is omitted.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"session_id": map[string]any{"type": "string", "description": "Session ID (optional, auto-detects if omitted)"},
			},
		},
	}
}

func toolGetBurnRate() ToolDefinition {
	return ToolDefinition{
		Name:        "get_burn_rate",
		Description: "Get token burn rate (consumption speed) for a session over a time window.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"session_id": map[string]any{"type": "string", "description": "Session ID (optional, auto-detects if omitted)"},
				"window":     map[string]any{"type": "string", "description": "Time window duration (e.g. 5m, 10m, 1h). Default: 5m"},
			},
		},
	}
}

func toolGetBillingBlock() ToolDefinition {
	return ToolDefinition{
		Name:        "get_billing_block",
		Description: "Get the current 5-hour billing block usage and time remaining.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"session_id": map[string]any{"type": "string", "description": "Session ID (optional, auto-detects if omitted)"},
			},
		},
	}
}

func toolListSessions() ToolDefinition {
	return ToolDefinition{
		Name:        "list_sessions",
		Description: "List all discovered Claude Code sessions sorted by most recent activity.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{"type": "integer", "description": "Maximum number of sessions to return. Default: 10"},
				"sort":  map[string]any{"type": "string", "description": "Sort order. Currently only 'recent' is supported."},
			},
		},
	}
}

func toolGetSessionDetail() ToolDefinition {
	return ToolDefinition{
		Name:        "get_session_detail",
		Description: "Get full stats, burn rate, and billing block for a specific session.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"session_id"},
			"properties": map[string]any{
				"session_id": map[string]any{"type": "string", "description": "Session ID (required)"},
			},
		},
	}
}

func toolCompareSessions() ToolDefinition {
	return ToolDefinition{
		Name:        "compare_sessions",
		Description: "Compare token usage statistics between two sessions.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"session_a", "session_b"},
			"properties": map[string]any{
				"session_a": map[string]any{"type": "string", "description": "First session ID"},
				"session_b": map[string]any{"type": "string", "description": "Second session ID"},
			},
		},
	}
}

// --- Handler helpers ---

// aggregateSession reads and aggregates a single session file.
func (c *sessionContext) aggregateSession(sf discovery.SessionFile) (aggregator.Aggregator, error) {
	r, err := c.readerFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer r.Close() //nolint:errcheck

	entries, err := r.Read(context.Background(), sf.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	agg := aggregator.New(aggregator.Config{})
	for _, e := range entries {
		agg.Add(e)
	}

	return agg, nil
}

// resolveSession finds a session file by ID, or auto-detects current.
func (c *sessionContext) resolveSession(sessionID string) (discovery.SessionFile, error) {
	if sessionID == "" {
		sf, err := c.disc.FindCurrentSession()
		if err != nil {
			return discovery.SessionFile{}, fmt.Errorf("failed to detect current session: %w", err)
		}
		return *sf, nil
	}

	sessions, err := c.disc.Discover()
	if err != nil {
		return discovery.SessionFile{}, fmt.Errorf("failed to discover sessions: %w", err)
	}

	for _, s := range sessions {
		if s.SessionID == sessionID {
			return s, nil
		}
	}

	return discovery.SessionFile{}, fmt.Errorf("session not found: %s", sessionID)
}

// textResult encodes a value as JSON and wraps it in a ToolCallResult.
func textResult(v any) (*ToolCallResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return &ToolCallResult{
		Content: []ToolContent{{Type: "text", Text: string(data)}},
	}, nil
}

// --- Tool handlers ---

func (c *sessionContext) handleGetTokenUsage(args json.RawMessage) (*ToolCallResult, error) {
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

	agg, err := c.aggregateSession(sf)
	if err != nil {
		return nil, err
	}

	stats := agg.Stats()
	return textResult(map[string]any{
		"session_id":    sf.SessionID,
		"total_tokens":  stats.TotalTokens,
		"input_tokens":  stats.InputTokens,
		"output_tokens": stats.OutputTokens,
		"count":         stats.Count,
		"avg_tokens":    stats.AvgTokens,
	})
}

func (c *sessionContext) handleGetBurnRate(args json.RawMessage) (*ToolCallResult, error) {
	var params struct {
		SessionID string `json:"session_id"`
		Window    string `json:"window"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	window := 5 * time.Minute
	if params.Window != "" {
		parsed, err := time.ParseDuration(params.Window)
		if err != nil {
			return nil, fmt.Errorf("invalid window duration %q: %w", params.Window, err)
		}
		window = parsed
	}

	sf, err := c.resolveSession(params.SessionID)
	if err != nil {
		return nil, err
	}

	agg, err := c.aggregateSession(sf)
	if err != nil {
		return nil, err
	}

	rate := agg.BurnRate(sf.SessionID, window)
	return textResult(map[string]any{
		"session_id":     sf.SessionID,
		"tokens_per_min": rate.TokensPerMinute,
		"tokens_per_hour": rate.TokensPerHour,
		"input_per_min":  rate.InputTokensPerMinute,
		"output_per_min": rate.OutputTokensPerMinute,
		"entry_count":    rate.EntryCount,
	})
}

func (c *sessionContext) handleGetBillingBlock(args json.RawMessage) (*ToolCallResult, error) {
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

	agg, err := c.aggregateSession(sf)
	if err != nil {
		return nil, err
	}

	block := agg.CurrentBillingBlock(sf.SessionID)
	remaining := time.Until(block.EndTime)
	if remaining < 0 {
		remaining = 0
	}

	return textResult(map[string]any{
		"session_id":     sf.SessionID,
		"start_time":     block.StartTime.UTC().Format(time.RFC3339),
		"end_time":       block.EndTime.UTC().Format(time.RFC3339),
		"total_tokens":   block.TotalTokens,
		"input_tokens":   block.InputTokens,
		"output_tokens":  block.OutputTokens,
		"entry_count":    block.EntryCount,
		"time_remaining": remaining.Round(time.Second).String(),
		"is_active":      block.IsActive,
	})
}

func (c *sessionContext) handleListSessions(args json.RawMessage) (*ToolCallResult, error) {
	var params struct {
		Limit int    `json:"limit"`
		Sort  string `json:"sort"`
	}
	params.Limit = 10
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}
	if params.Limit <= 0 {
		params.Limit = 10
	}

	sessions, err := c.disc.Discover()
	if err != nil {
		return nil, fmt.Errorf("failed to discover sessions: %w", err)
	}

	// Sort by ModTime descending (most recent first).
	sorted := make([]discovery.SessionFile, len(sessions))
	copy(sorted, sessions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ModTime > sorted[j].ModTime
	})

	if params.Limit < len(sorted) {
		sorted = sorted[:params.Limit]
	}

	items := make([]map[string]any, 0, len(sorted))
	for _, s := range sorted {
		items = append(items, map[string]any{
			"session_id":    s.SessionID,
			"project_path":  s.ProjectPath,
			"size":          s.Size,
			"last_modified": time.Unix(s.ModTime, 0).UTC().Format(time.RFC3339),
		})
	}

	return textResult(items)
}

func (c *sessionContext) handleGetSessionDetail(args json.RawMessage) (*ToolCallResult, error) {
	var params struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if params.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	sf, err := c.resolveSession(params.SessionID)
	if err != nil {
		return nil, err
	}

	agg, err := c.aggregateSession(sf)
	if err != nil {
		return nil, err
	}

	stats := agg.Stats()
	rate := agg.BurnRate(sf.SessionID, 5*time.Minute)
	block := agg.CurrentBillingBlock(sf.SessionID)

	remaining := time.Until(block.EndTime)
	if remaining < 0 {
		remaining = 0
	}

	return textResult(map[string]any{
		"session_id": sf.SessionID,
		"stats": map[string]any{
			"total_tokens":  stats.TotalTokens,
			"input_tokens":  stats.InputTokens,
			"output_tokens": stats.OutputTokens,
			"count":         stats.Count,
			"avg_tokens":    stats.AvgTokens,
		},
		"burn_rate": map[string]any{
			"tokens_per_min":  rate.TokensPerMinute,
			"tokens_per_hour": rate.TokensPerHour,
			"input_per_min":   rate.InputTokensPerMinute,
			"output_per_min":  rate.OutputTokensPerMinute,
			"entry_count":     rate.EntryCount,
		},
		"billing_block": map[string]any{
			"start_time":     block.StartTime.UTC().Format(time.RFC3339),
			"end_time":       block.EndTime.UTC().Format(time.RFC3339),
			"total_tokens":   block.TotalTokens,
			"input_tokens":   block.InputTokens,
			"output_tokens":  block.OutputTokens,
			"entry_count":    block.EntryCount,
			"time_remaining": remaining.Round(time.Second).String(),
			"is_active":      block.IsActive,
		},
	})
}

func (c *sessionContext) handleCompareSessions(args json.RawMessage) (*ToolCallResult, error) {
	var params struct {
		SessionA string `json:"session_a"`
		SessionB string `json:"session_b"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if params.SessionA == "" || params.SessionB == "" {
		return nil, fmt.Errorf("session_a and session_b are required")
	}

	sfA, err := c.resolveSession(params.SessionA)
	if err != nil {
		return nil, fmt.Errorf("session_a: %w", err)
	}

	sfB, err := c.resolveSession(params.SessionB)
	if err != nil {
		return nil, fmt.Errorf("session_b: %w", err)
	}

	aggA, err := c.aggregateSession(sfA)
	if err != nil {
		return nil, fmt.Errorf("session_a read error: %w", err)
	}

	aggB, err := c.aggregateSession(sfB)
	if err != nil {
		return nil, fmt.Errorf("session_b read error: %w", err)
	}

	statsA := aggA.Stats()
	statsB := aggB.Stats()

	sessionStats := func(id string, s aggregator.Statistics) map[string]any {
		return map[string]any{
			"session_id":    id,
			"total_tokens":  s.TotalTokens,
			"input_tokens":  s.InputTokens,
			"output_tokens": s.OutputTokens,
			"count":         s.Count,
			"avg_tokens":    s.AvgTokens,
		}
	}

	return textResult(map[string]any{
		"session_a": sessionStats(sfA.SessionID, statsA),
		"session_b": sessionStats(sfB.SessionID, statsB),
		"diff": map[string]any{
			"total_tokens":  statsB.TotalTokens - statsA.TotalTokens,
			"input_tokens":  statsB.InputTokens - statsA.InputTokens,
			"output_tokens": statsB.OutputTokens - statsA.OutputTokens,
			"count":         statsB.Count - statsA.Count,
		},
	})
}
