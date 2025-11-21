package monitor

import (
	"time"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
)

// Config holds the configuration for the live monitor.
type Config struct {
	// SessionIDs to monitor (empty means all sessions)
	SessionIDs []string

	// RefreshInterval is the interval between display updates
	RefreshInterval time.Duration

	// ClearScreen enables clearing the terminal between updates
	ClearScreen bool
}

// LiveMonitor provides real-time token usage monitoring.
type LiveMonitor interface {
	// Start begins monitoring and blocks until stopped
	Start() error

	// Stop stops the monitor gracefully
	Stop() error

	// Stats returns the current statistics
	Stats() aggregator.Statistics
}

// Update represents a live monitoring update event.
type Update struct {
	// Timestamp of the update
	Timestamp time.Time

	// Stats contains the current aggregated statistics
	Stats aggregator.Statistics

	// Delta contains the change since last update
	Delta DeltaStats

	// Cumulative contains the total change since monitor started
	Cumulative DeltaStats

	// SessionID being monitored (empty if all sessions)
	SessionID string

	// BurnRate contains token consumption rate metrics
	BurnRate aggregator.BurnRate

	// CurrentBlock contains the current billing block stats
	CurrentBlock aggregator.BillingBlock
}

// DeltaStats represents changes since the last update.
type DeltaStats struct {
	// NewEntries is the number of new entries processed
	NewEntries int

	// InputTokens added since last update
	InputTokens int

	// OutputTokens added since last update
	OutputTokens int

	// TotalTokens added since last update
	TotalTokens int
}
