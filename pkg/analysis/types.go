// Package analysis provides session analysis and comparison functionality.
// It analyzes parsed JSONL entries to produce per-turn breakdowns,
// tool usage statistics, cache efficiency metrics, and cost estimations.
package analysis

import "time"

// SessionAnalysis holds comprehensive analysis data for a single session.
type SessionAnalysis struct {
	SessionID string
	Label     string
	Project   string

	// Overall metrics
	EntryCount int
	FirstSeen  time.Time
	LastSeen   time.Time
	Duration   time.Duration

	// Token totals
	InputTokens   int
	OutputTokens  int
	CacheCreation int
	CacheRead     int
	TotalTokens   int
	RealInput     int // input + cache_creation + cache_read

	// Per-turn data
	Turns []TurnData

	// Tool usage frequency
	ToolUsage map[string]int

	// Cache efficiency
	CacheHitRate float64

	// Cost estimation
	CostUSD     float64
	HasRealCost bool // true if cost came from JSONL costUSD field

	// Models used
	Models map[string]int
}

// TurnData holds per-turn analysis data.
type TurnData struct {
	Turn          int
	Timestamp     time.Time
	Model         string
	InputTokens   int
	OutputTokens  int
	CacheCreation int
	CacheRead     int
	TotalTokens   int
	RealInput     int
	Tools         []string
	CostUSD       *float64
}

// ModelPricing defines token pricing for a model (per million tokens).
type ModelPricing struct {
	InputPerMTok      float64
	OutputPerMTok     float64
	CacheWritePerMTok float64
	CacheReadPerMTok  float64
}
