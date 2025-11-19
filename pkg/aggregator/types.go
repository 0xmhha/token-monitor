// Package aggregator provides token usage aggregation and statistics.
//
// It aggregates usage data across sessions, models, and time windows,
// providing summary statistics and insights.
//
// Example usage:
//
//	agg := aggregator.New(aggregator.Config{
//	    GroupBy: []aggregator.Dimension{aggregator.DimModel, aggregator.DimSession},
//	})
//
//	// Add usage entries
//	for _, entry := range entries {
//	    agg.Add(entry)
//	}
//
//	// Get statistics
//	stats := agg.Stats()
//	fmt.Printf("Total tokens: %d\n", stats.TotalTokens)
//	fmt.Printf("Sessions: %d\n", stats.SessionCount)
package aggregator

import (
	"time"

	"github.com/yourusername/token-monitor/pkg/parser"
)

// Dimension represents an aggregation dimension.
type Dimension string

const (
	// DimModel aggregates by model name.
	DimModel Dimension = "model"

	// DimSession aggregates by session ID.
	DimSession Dimension = "session"

	// DimDate aggregates by date (YYYY-MM-DD).
	DimDate Dimension = "date"

	// DimHour aggregates by hour (YYYY-MM-DD HH:00).
	DimHour Dimension = "hour"
)

// Aggregator computes token usage statistics.
type Aggregator interface {
	// Add adds a usage entry to the aggregator.
	//
	// Parameters:
	//   - entry: Usage entry to aggregate
	Add(entry parser.UsageEntry)

	// Stats returns aggregated statistics.
	//
	// Returns overall statistics across all entries.
	Stats() Statistics

	// GroupedStats returns statistics grouped by configured dimensions.
	//
	// Returns:
	//   - Map of dimension values to statistics
	//
	// For example, if GroupBy is [DimModel], returns stats per model.
	// If GroupBy is [DimModel, DimSession], returns stats per model+session.
	GroupedStats() map[string]Statistics

	// TopSessions returns top N sessions by token usage.
	//
	// Parameters:
	//   - n: Number of top sessions to return
	//
	// Returns:
	//   - Slice of session statistics, sorted by total tokens descending
	TopSessions(n int) []SessionStats

	// BurnRate calculates token consumption rate over a time window.
	//
	// Parameters:
	//   - sessionID: Session to calculate rate for (empty for all sessions)
	//   - window: Time window duration for rate calculation
	//
	// Returns:
	//   - BurnRate metrics for the specified window
	BurnRate(sessionID string, window time.Duration) BurnRate

	// BillingBlocks returns token usage grouped by 5-hour billing windows.
	//
	// Parameters:
	//   - sessionID: Session to get blocks for (empty for all sessions)
	//
	// Returns:
	//   - Slice of billing blocks sorted by start time (most recent first)
	BillingBlocks(sessionID string) []BillingBlock

	// CurrentBillingBlock returns the current active billing block.
	//
	// Parameters:
	//   - sessionID: Session to get block for (empty for all sessions)
	//
	// Returns:
	//   - Current billing block with usage so far
	CurrentBillingBlock(sessionID string) BillingBlock

	// Reset clears all aggregated data.
	Reset()
}

// Statistics contains aggregated token usage statistics.
type Statistics struct {
	// Count is the number of entries.
	Count int

	// SessionCount is the number of unique sessions.
	SessionCount int

	// TotalTokens is the sum of all tokens.
	TotalTokens int

	// InputTokens is the sum of all input tokens.
	InputTokens int

	// OutputTokens is the sum of all output tokens.
	OutputTokens int

	// AvgTokens is the average tokens per entry.
	AvgTokens float64

	// MinTokens is the minimum tokens in any entry.
	MinTokens int

	// MaxTokens is the maximum tokens in any entry.
	MaxTokens int

	// P50Tokens is the 50th percentile (median) tokens.
	P50Tokens int

	// P95Tokens is the 95th percentile tokens.
	P95Tokens int

	// P99Tokens is the 99th percentile tokens.
	P99Tokens int

	// FirstSeen is the timestamp of the first entry.
	FirstSeen time.Time

	// LastSeen is the timestamp of the last entry.
	LastSeen time.Time
}

// SessionStats contains statistics for a single session.
type SessionStats struct {
	// SessionID is the session identifier.
	SessionID string

	// Model is the model name.
	Model string

	// Statistics contains aggregated stats for this session.
	Statistics Statistics
}

// BurnRate contains token consumption rate metrics.
type BurnRate struct {
	// TokensPerMinute is the average token consumption rate.
	TokensPerMinute float64

	// TokensPerHour is the hourly projection.
	TokensPerHour float64

	// InputTokensPerMinute is input token rate.
	InputTokensPerMinute float64

	// OutputTokensPerMinute is output token rate.
	OutputTokensPerMinute float64

	// WindowDuration is the time window used for calculation.
	WindowDuration time.Duration

	// EntryCount is number of entries in the window.
	EntryCount int

	// TotalTokens is total tokens in the window.
	TotalTokens int

	// ProjectedHourlyTokens is projected tokens for one hour.
	ProjectedHourlyTokens int
}

// TimestampedEntry stores an entry with its timestamp for burn rate calculation.
type TimestampedEntry struct {
	Timestamp    time.Time
	TotalTokens  int
	InputTokens  int
	OutputTokens int
	SessionID    string
}

// BillingBlock represents a 5-hour billing window for Claude API.
// Billing blocks are aligned to UTC: 00:00-05:00, 05:00-10:00, etc.
type BillingBlock struct {
	// StartTime is the UTC start of this billing block.
	StartTime time.Time

	// EndTime is the UTC end of this billing block.
	EndTime time.Time

	// TotalTokens in this block.
	TotalTokens int

	// InputTokens in this block.
	InputTokens int

	// OutputTokens in this block.
	OutputTokens int

	// EntryCount is the number of entries in this block.
	EntryCount int

	// IsActive indicates if this is the current billing block.
	IsActive bool
}

// Config contains aggregator configuration.
type Config struct {
	// GroupBy specifies aggregation dimensions.
	//
	// Examples:
	//   - [DimModel] - aggregate by model
	//   - [DimModel, DimSession] - aggregate by model and session
	//   - [DimDate] - aggregate by date
	//
	// Default: no grouping (overall stats only).
	GroupBy []Dimension

	// TrackPercentiles enables percentile calculation.
	//
	// Percentile calculation requires storing all token counts in memory,
	// so disable if memory is a concern.
	//
	// Default: true.
	TrackPercentiles bool
}
