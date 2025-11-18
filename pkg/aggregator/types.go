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
