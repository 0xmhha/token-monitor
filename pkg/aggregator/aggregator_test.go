package aggregator

import (
	"testing"
	"time"

	"github.com/0xmhha/token-monitor/pkg/parser"
)

func TestNew(t *testing.T) {
	t.Parallel()

	agg := New(Config{})
	if agg == nil {
		t.Fatal("New() returned nil")
	}
}

func TestAdd_SingleEntry(t *testing.T) {
	t.Parallel()

	agg := New(Config{TrackPercentiles: true})

	entry := parser.UsageEntry{
		SessionID: "session-1",
		Timestamp: time.Now(),
		Message: parser.Message{
			Model: "claude-3-5-sonnet-20241022",
			Usage: parser.Usage{
				InputTokens:  100,
				OutputTokens: 50,
			},
		},
	}

	agg.Add(entry)

	stats := agg.Stats()
	if stats.Count != 1 {
		t.Errorf("Stats().Count = %d, want 1", stats.Count)
	}
	if stats.TotalTokens != 150 {
		t.Errorf("Stats().TotalTokens = %d, want 150", stats.TotalTokens)
	}
	if stats.InputTokens != 100 {
		t.Errorf("Stats().InputTokens = %d, want 100", stats.InputTokens)
	}
	if stats.OutputTokens != 50 {
		t.Errorf("Stats().OutputTokens = %d, want 50", stats.OutputTokens)
	}
	if stats.AvgTokens != 150.0 {
		t.Errorf("Stats().AvgTokens = %f, want 150.0", stats.AvgTokens)
	}
	if stats.MinTokens != 150 {
		t.Errorf("Stats().MinTokens = %d, want 150", stats.MinTokens)
	}
	if stats.MaxTokens != 150 {
		t.Errorf("Stats().MaxTokens = %d, want 150", stats.MaxTokens)
	}
}

func TestAdd_MultipleEntries(t *testing.T) {
	t.Parallel()

	agg := New(Config{TrackPercentiles: true})

	entries := []parser.UsageEntry{
		{
			SessionID: "session-1",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
		{
			SessionID: "session-1",
			Timestamp: time.Now().Add(1 * time.Minute),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
		{
			SessionID: "session-2",
			Timestamp: time.Now().Add(2 * time.Minute),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 150, OutputTokens: 75},
			},
		},
	}

	for _, entry := range entries {
		agg.Add(entry)
	}

	stats := agg.Stats()
	if stats.Count != 3 {
		t.Errorf("Stats().Count = %d, want 3", stats.Count)
	}
	if stats.TotalTokens != 675 {
		t.Errorf("Stats().TotalTokens = %d, want 675", stats.TotalTokens)
	}
	if stats.InputTokens != 450 {
		t.Errorf("Stats().InputTokens = %d, want 450", stats.InputTokens)
	}
	if stats.OutputTokens != 225 {
		t.Errorf("Stats().OutputTokens = %d, want 225", stats.OutputTokens)
	}
	if stats.AvgTokens != 225.0 {
		t.Errorf("Stats().AvgTokens = %f, want 225.0", stats.AvgTokens)
	}
	if stats.MinTokens != 150 {
		t.Errorf("Stats().MinTokens = %d, want 150", stats.MinTokens)
	}
	if stats.MaxTokens != 300 {
		t.Errorf("Stats().MaxTokens = %d, want 300", stats.MaxTokens)
	}
}

func TestGroupedStats_ByModel(t *testing.T) {
	t.Parallel()

	agg := New(Config{
		GroupBy:          []Dimension{DimModel},
		TrackPercentiles: true,
	})

	entries := []parser.UsageEntry{
		{
			SessionID: "session-1",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
		{
			SessionID: "session-2",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-opus-20240229",
				Usage: parser.Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
		{
			SessionID: "session-3",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 150, OutputTokens: 75},
			},
		},
	}

	for _, entry := range entries {
		agg.Add(entry)
	}

	grouped := agg.GroupedStats()
	if len(grouped) != 2 {
		t.Fatalf("GroupedStats() returned %d groups, want 2", len(grouped))
	}

	sonnetStats, exists := grouped["claude-3-5-sonnet-20241022"]
	if !exists {
		t.Fatal("GroupedStats() missing sonnet model")
	}
	if sonnetStats.Count != 2 {
		t.Errorf("Sonnet Count = %d, want 2", sonnetStats.Count)
	}
	if sonnetStats.TotalTokens != 375 {
		t.Errorf("Sonnet TotalTokens = %d, want 375", sonnetStats.TotalTokens)
	}

	opusStats, exists := grouped["claude-3-opus-20240229"]
	if !exists {
		t.Fatal("GroupedStats() missing opus model")
	}
	if opusStats.Count != 1 {
		t.Errorf("Opus Count = %d, want 1", opusStats.Count)
	}
	if opusStats.TotalTokens != 300 {
		t.Errorf("Opus TotalTokens = %d, want 300", opusStats.TotalTokens)
	}
}

func TestGroupedStats_BySession(t *testing.T) {
	t.Parallel()

	agg := New(Config{
		GroupBy:          []Dimension{DimSession},
		TrackPercentiles: true,
	})

	entries := []parser.UsageEntry{
		{
			SessionID: "session-1",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
		{
			SessionID: "session-1",
			Timestamp: time.Now().Add(1 * time.Minute),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
		{
			SessionID: "session-2",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 150, OutputTokens: 75},
			},
		},
	}

	for _, entry := range entries {
		agg.Add(entry)
	}

	grouped := agg.GroupedStats()
	if len(grouped) != 2 {
		t.Fatalf("GroupedStats() returned %d groups, want 2", len(grouped))
	}

	session1Stats, exists := grouped["session-1"]
	if !exists {
		t.Fatal("GroupedStats() missing session-1")
	}
	if session1Stats.Count != 2 {
		t.Errorf("Session1 Count = %d, want 2", session1Stats.Count)
	}
	if session1Stats.TotalTokens != 450 {
		t.Errorf("Session1 TotalTokens = %d, want 450", session1Stats.TotalTokens)
	}
}

func TestGroupedStats_ByModelAndSession(t *testing.T) {
	t.Parallel()

	agg := New(Config{
		GroupBy:          []Dimension{DimModel, DimSession},
		TrackPercentiles: true,
	})

	entries := []parser.UsageEntry{
		{
			SessionID: "session-1",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
		{
			SessionID: "session-1",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-opus-20240229",
				Usage: parser.Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
	}

	for _, entry := range entries {
		agg.Add(entry)
	}

	grouped := agg.GroupedStats()
	if len(grouped) != 2 {
		t.Fatalf("GroupedStats() returned %d groups, want 2", len(grouped))
	}

	key1 := "claude-3-5-sonnet-20241022|session-1"
	stats1, exists := grouped[key1]
	if !exists {
		t.Fatalf("GroupedStats() missing key %s", key1)
	}
	if stats1.TotalTokens != 150 {
		t.Errorf("Key %s TotalTokens = %d, want 150", key1, stats1.TotalTokens)
	}

	key2 := "claude-3-opus-20240229|session-1"
	stats2, exists := grouped[key2]
	if !exists {
		t.Fatalf("GroupedStats() missing key %s", key2)
	}
	if stats2.TotalTokens != 300 {
		t.Errorf("Key %s TotalTokens = %d, want 300", key2, stats2.TotalTokens)
	}
}

func TestTopSessions(t *testing.T) {
	t.Parallel()

	agg := New(Config{
		GroupBy:          []Dimension{DimSession, DimModel},
		TrackPercentiles: true,
	})

	entries := []parser.UsageEntry{
		{
			SessionID: "session-1",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 1000, OutputTokens: 500},
			},
		},
		{
			SessionID: "session-2",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 2000, OutputTokens: 1000},
			},
		},
		{
			SessionID: "session-3",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 500, OutputTokens: 250},
			},
		},
	}

	for _, entry := range entries {
		agg.Add(entry)
	}

	topSessions := agg.TopSessions(2)
	if len(topSessions) != 2 {
		t.Fatalf("TopSessions(2) returned %d sessions, want 2", len(topSessions))
	}

	// Should be sorted by total tokens descending.
	if topSessions[0].SessionID != "session-2" {
		t.Errorf("TopSessions[0].SessionID = %s, want session-2", topSessions[0].SessionID)
	}
	if topSessions[0].Statistics.TotalTokens != 3000 {
		t.Errorf("TopSessions[0].TotalTokens = %d, want 3000", topSessions[0].Statistics.TotalTokens)
	}

	if topSessions[1].SessionID != "session-1" {
		t.Errorf("TopSessions[1].SessionID = %s, want session-1", topSessions[1].SessionID)
	}
	if topSessions[1].Statistics.TotalTokens != 1500 {
		t.Errorf("TopSessions[1].TotalTokens = %d, want 1500", topSessions[1].Statistics.TotalTokens)
	}
}

func TestPercentiles(t *testing.T) {
	t.Parallel()

	agg := New(Config{TrackPercentiles: true})

	// Add entries with known distribution: 100, 150, 200, 250, 300
	entries := []parser.UsageEntry{
		{
			SessionID: "session-1",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 100, OutputTokens: 0},
			},
		},
		{
			SessionID: "session-1",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 150, OutputTokens: 0},
			},
		},
		{
			SessionID: "session-1",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 200, OutputTokens: 0},
			},
		},
		{
			SessionID: "session-1",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 250, OutputTokens: 0},
			},
		},
		{
			SessionID: "session-1",
			Timestamp: time.Now(),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 300, OutputTokens: 0},
			},
		},
	}

	for _, entry := range entries {
		agg.Add(entry)
	}

	stats := agg.Stats()

	// P50 should be around 200 (median).
	if stats.P50Tokens != 200 {
		t.Errorf("Stats().P50Tokens = %d, want 200", stats.P50Tokens)
	}

	// P95 should be around 290.
	if stats.P95Tokens < 280 || stats.P95Tokens > 300 {
		t.Errorf("Stats().P95Tokens = %d, want ~290", stats.P95Tokens)
	}

	// P99 should be around 298.
	if stats.P99Tokens < 290 || stats.P99Tokens > 300 {
		t.Errorf("Stats().P99Tokens = %d, want ~298", stats.P99Tokens)
	}
}

func TestReset(t *testing.T) {
	t.Parallel()

	agg := New(Config{
		GroupBy:          []Dimension{DimModel},
		TrackPercentiles: true,
	})

	entry := parser.UsageEntry{
		SessionID: "session-1",
		Timestamp: time.Now(),
		Message: parser.Message{
			Model: "claude-3-5-sonnet-20241022",
			Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
		},
	}

	agg.Add(entry)

	stats := agg.Stats()
	if stats.Count != 1 {
		t.Errorf("Stats().Count = %d, want 1 before reset", stats.Count)
	}

	agg.Reset()

	stats = agg.Stats()
	if stats.Count != 0 {
		t.Errorf("Stats().Count = %d, want 0 after reset", stats.Count)
	}
	if stats.TotalTokens != 0 {
		t.Errorf("Stats().TotalTokens = %d, want 0 after reset", stats.TotalTokens)
	}

	grouped := agg.GroupedStats()
	if len(grouped) != 0 {
		t.Errorf("GroupedStats() returned %d groups after reset, want 0", len(grouped))
	}
}

func TestConcurrency(t *testing.T) {
	t.Parallel()

	agg := New(Config{TrackPercentiles: true})

	const goroutines = 10
	const entriesPerGoroutine = 100

	done := make(chan bool)

	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < entriesPerGoroutine; j++ {
				entry := parser.UsageEntry{
					SessionID: "session-1",
					Timestamp: time.Now(),
					Message: parser.Message{
						Model: "claude-3-5-sonnet-20241022",
						Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
					},
				}
				agg.Add(entry)
			}
			done <- true
		}()
	}

	// Wait for all goroutines.
	for i := 0; i < goroutines; i++ {
		<-done
	}

	stats := agg.Stats()
	expectedCount := goroutines * entriesPerGoroutine
	if stats.Count != expectedCount {
		t.Errorf("Stats().Count = %d, want %d", stats.Count, expectedCount)
	}
}

func TestGroupedStats_ByDate(t *testing.T) {
	t.Parallel()

	agg := New(Config{
		GroupBy:          []Dimension{DimDate},
		TrackPercentiles: true,
	})

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	entries := []parser.UsageEntry{
		{
			SessionID: "session-1",
			Timestamp: now,
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
		{
			SessionID: "session-2",
			Timestamp: yesterday,
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
		{
			SessionID: "session-3",
			Timestamp: now,
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 150, OutputTokens: 75},
			},
		},
	}

	for _, entry := range entries {
		agg.Add(entry)
	}

	grouped := agg.GroupedStats()
	if len(grouped) != 2 {
		t.Fatalf("GroupedStats() returned %d groups, want 2", len(grouped))
	}

	todayKey := now.Format("2006-01-02")
	todayStats, exists := grouped[todayKey]
	if !exists {
		t.Fatalf("GroupedStats() missing today's date %s", todayKey)
	}
	if todayStats.Count != 2 {
		t.Errorf("Today Count = %d, want 2", todayStats.Count)
	}

	yesterdayKey := yesterday.Format("2006-01-02")
	yesterdayStats, exists := grouped[yesterdayKey]
	if !exists {
		t.Fatalf("GroupedStats() missing yesterday's date %s", yesterdayKey)
	}
	if yesterdayStats.Count != 1 {
		t.Errorf("Yesterday Count = %d, want 1", yesterdayStats.Count)
	}
}

func TestGroupedStats_ByHour(t *testing.T) {
	t.Parallel()

	agg := New(Config{
		GroupBy:          []Dimension{DimHour},
		TrackPercentiles: true,
	})

	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)

	entries := []parser.UsageEntry{
		{
			SessionID: "session-1",
			Timestamp: now,
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
		{
			SessionID: "session-2",
			Timestamp: hourAgo,
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
		{
			SessionID: "session-3",
			Timestamp: now,
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 150, OutputTokens: 75},
			},
		},
	}

	for _, entry := range entries {
		agg.Add(entry)
	}

	grouped := agg.GroupedStats()
	if len(grouped) != 2 {
		t.Fatalf("GroupedStats() returned %d groups, want 2", len(grouped))
	}

	currentHourKey := now.Format("2006-01-02 15:00")
	currentHourStats, exists := grouped[currentHourKey]
	if !exists {
		t.Fatalf("GroupedStats() missing current hour %s", currentHourKey)
	}
	if currentHourStats.Count != 2 {
		t.Errorf("Current hour Count = %d, want 2", currentHourStats.Count)
	}
}


func TestBurnRate_EmptyAggregator(t *testing.T) {
	t.Parallel()

	agg := New(Config{})

	rate := agg.BurnRate("", 5*time.Minute)
	if rate.EntryCount != 0 {
		t.Errorf("BurnRate().EntryCount = %d, want 0", rate.EntryCount)
	}
	if rate.TokensPerMinute != 0 {
		t.Errorf("BurnRate().TokensPerMinute = %f, want 0", rate.TokensPerMinute)
	}
}

func TestBurnRate_AllSessions(t *testing.T) {
	t.Parallel()

	agg := New(Config{})

	now := time.Now()
	entries := []parser.UsageEntry{
		{
			SessionID: "session-1",
			Timestamp: now.Add(-4 * time.Minute),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
		{
			SessionID: "session-2",
			Timestamp: now.Add(-2 * time.Minute),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
		{
			SessionID: "session-1",
			Timestamp: now.Add(-1 * time.Minute),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 150, OutputTokens: 75},
			},
		},
	}

	for _, entry := range entries {
		agg.Add(entry)
	}

	// Calculate burn rate over 5 minute window
	rate := agg.BurnRate("", 5*time.Minute)

	if rate.EntryCount != 3 {
		t.Errorf("BurnRate().EntryCount = %d, want 3", rate.EntryCount)
	}
	if rate.TotalTokens != 675 {
		t.Errorf("BurnRate().TotalTokens = %d, want 675", rate.TotalTokens)
	}

	// 675 tokens / 5 minutes = 135 tokens per minute
	expectedRate := 135.0
	if rate.TokensPerMinute != expectedRate {
		t.Errorf("BurnRate().TokensPerMinute = %f, want %f", rate.TokensPerMinute, expectedRate)
	}

	// 135 * 60 = 8100 tokens per hour
	if rate.TokensPerHour != 8100.0 {
		t.Errorf("BurnRate().TokensPerHour = %f, want 8100", rate.TokensPerHour)
	}

	if rate.ProjectedHourlyTokens != 8100 {
		t.Errorf("BurnRate().ProjectedHourlyTokens = %d, want 8100", rate.ProjectedHourlyTokens)
	}
}

func TestBurnRate_SpecificSession(t *testing.T) {
	t.Parallel()

	agg := New(Config{})

	now := time.Now()
	entries := []parser.UsageEntry{
		{
			SessionID: "session-1",
			Timestamp: now.Add(-4 * time.Minute),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
		{
			SessionID: "session-2",
			Timestamp: now.Add(-2 * time.Minute),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
		{
			SessionID: "session-1",
			Timestamp: now.Add(-1 * time.Minute),
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 150, OutputTokens: 75},
			},
		},
	}

	for _, entry := range entries {
		agg.Add(entry)
	}

	// Calculate burn rate for session-1 only
	rate := agg.BurnRate("session-1", 5*time.Minute)

	if rate.EntryCount != 2 {
		t.Errorf("BurnRate().EntryCount = %d, want 2", rate.EntryCount)
	}
	if rate.TotalTokens != 375 {
		t.Errorf("BurnRate().TotalTokens = %d, want 375", rate.TotalTokens)
	}

	// 375 tokens / 5 minutes = 75 tokens per minute
	expectedRate := 75.0
	if rate.TokensPerMinute != expectedRate {
		t.Errorf("BurnRate().TokensPerMinute = %f, want %f", rate.TokensPerMinute, expectedRate)
	}
}

func TestBurnRate_WindowFiltering(t *testing.T) {
	t.Parallel()

	agg := New(Config{})

	now := time.Now()
	entries := []parser.UsageEntry{
		{
			SessionID: "session-1",
			Timestamp: now.Add(-10 * time.Minute), // Outside window
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 1000, OutputTokens: 500},
			},
		},
		{
			SessionID: "session-1",
			Timestamp: now.Add(-2 * time.Minute), // Inside window
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
	}

	for _, entry := range entries {
		agg.Add(entry)
	}

	// Calculate burn rate over 5 minute window (should exclude old entry)
	rate := agg.BurnRate("", 5*time.Minute)

	if rate.EntryCount != 1 {
		t.Errorf("BurnRate().EntryCount = %d, want 1", rate.EntryCount)
	}
	if rate.TotalTokens != 150 {
		t.Errorf("BurnRate().TotalTokens = %d, want 150", rate.TotalTokens)
	}
}

func TestBurnRate_InputOutputBreakdown(t *testing.T) {
	t.Parallel()

	agg := New(Config{})

	now := time.Now()
	entry := parser.UsageEntry{
		SessionID: "session-1",
		Timestamp: now.Add(-2 * time.Minute),
		Message: parser.Message{
			Model: "claude-3-5-sonnet-20241022",
			Usage: parser.Usage{InputTokens: 200, OutputTokens: 100},
		},
	}

	agg.Add(entry)

	rate := agg.BurnRate("", 5*time.Minute)

	// 200 input / 5 min = 40 per minute
	if rate.InputTokensPerMinute != 40.0 {
		t.Errorf("BurnRate().InputTokensPerMinute = %f, want 40", rate.InputTokensPerMinute)
	}

	// 100 output / 5 min = 20 per minute
	if rate.OutputTokensPerMinute != 20.0 {
		t.Errorf("BurnRate().OutputTokensPerMinute = %f, want 20", rate.OutputTokensPerMinute)
	}
}

func TestBurnRate_Reset(t *testing.T) {
	t.Parallel()

	agg := New(Config{})

	now := time.Now()
	entry := parser.UsageEntry{
		SessionID: "session-1",
		Timestamp: now.Add(-1 * time.Minute),
		Message: parser.Message{
			Model: "claude-3-5-sonnet-20241022",
			Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
		},
	}

	agg.Add(entry)

	// Verify entry exists
	rate := agg.BurnRate("", 5*time.Minute)
	if rate.EntryCount != 1 {
		t.Errorf("Before reset: BurnRate().EntryCount = %d, want 1", rate.EntryCount)
	}

	// Reset
	agg.Reset()

	// Verify entry is gone
	rate = agg.BurnRate("", 5*time.Minute)
	if rate.EntryCount != 0 {
		t.Errorf("After reset: BurnRate().EntryCount = %d, want 0", rate.EntryCount)
	}
}

func TestBillingBlocks_Empty(t *testing.T) {
	t.Parallel()

	agg := New(Config{})

	blocks := agg.BillingBlocks("")
	if blocks != nil {
		t.Errorf("BillingBlocks() = %v, want nil", blocks)
	}
}

func TestBillingBlocks_SingleBlock(t *testing.T) {
	t.Parallel()

	agg := New(Config{})

	now := time.Now()
	entry := parser.UsageEntry{
		SessionID: "session-1",
		Timestamp: now,
		Message: parser.Message{
			Model: "claude-3-5-sonnet-20241022",
			Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
		},
	}

	agg.Add(entry)

	blocks := agg.BillingBlocks("")
	if len(blocks) != 1 {
		t.Fatalf("BillingBlocks() returned %d blocks, want 1", len(blocks))
	}

	if blocks[0].TotalTokens != 150 {
		t.Errorf("Block.TotalTokens = %d, want 150", blocks[0].TotalTokens)
	}
	if blocks[0].EntryCount != 1 {
		t.Errorf("Block.EntryCount = %d, want 1", blocks[0].EntryCount)
	}
	if !blocks[0].IsActive {
		t.Error("Block.IsActive = false, want true")
	}
}

func TestBillingBlocks_MultipleBlocks(t *testing.T) {
	t.Parallel()

	agg := New(Config{})

	now := time.Now().UTC()
	// Entry in current block
	entry1 := parser.UsageEntry{
		SessionID: "session-1",
		Timestamp: now,
		Message: parser.Message{
			Model: "claude-3-5-sonnet-20241022",
			Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
		},
	}
	// Entry 6 hours ago (different block)
	entry2 := parser.UsageEntry{
		SessionID: "session-1",
		Timestamp: now.Add(-6 * time.Hour),
		Message: parser.Message{
			Model: "claude-3-5-sonnet-20241022",
			Usage: parser.Usage{InputTokens: 200, OutputTokens: 100},
		},
	}

	agg.Add(entry1)
	agg.Add(entry2)

	blocks := agg.BillingBlocks("")
	if len(blocks) != 2 {
		t.Fatalf("BillingBlocks() returned %d blocks, want 2", len(blocks))
	}

	// Most recent block should be first
	if !blocks[0].IsActive {
		t.Error("First block should be active")
	}
	if blocks[0].TotalTokens != 150 {
		t.Errorf("Active block TotalTokens = %d, want 150", blocks[0].TotalTokens)
	}
	if blocks[1].TotalTokens != 300 {
		t.Errorf("Previous block TotalTokens = %d, want 300", blocks[1].TotalTokens)
	}
}

func TestBillingBlocks_SessionFilter(t *testing.T) {
	t.Parallel()

	agg := New(Config{})

	now := time.Now()
	entries := []parser.UsageEntry{
		{
			SessionID: "session-1",
			Timestamp: now,
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
		{
			SessionID: "session-2",
			Timestamp: now,
			Message: parser.Message{
				Model: "claude-3-5-sonnet-20241022",
				Usage: parser.Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
	}

	for _, entry := range entries {
		agg.Add(entry)
	}

	// Filter by session-1
	blocks := agg.BillingBlocks("session-1")
	if len(blocks) != 1 {
		t.Fatalf("BillingBlocks() returned %d blocks, want 1", len(blocks))
	}
	if blocks[0].TotalTokens != 150 {
		t.Errorf("Block.TotalTokens = %d, want 150", blocks[0].TotalTokens)
	}
}

func TestCurrentBillingBlock(t *testing.T) {
	t.Parallel()

	agg := New(Config{})

	now := time.Now()
	// Entry in current block
	entry1 := parser.UsageEntry{
		SessionID: "session-1",
		Timestamp: now,
		Message: parser.Message{
			Model: "claude-3-5-sonnet-20241022",
			Usage: parser.Usage{InputTokens: 100, OutputTokens: 50},
		},
	}
	// Entry 6 hours ago (different block - should be excluded)
	entry2 := parser.UsageEntry{
		SessionID: "session-1",
		Timestamp: now.Add(-6 * time.Hour),
		Message: parser.Message{
			Model: "claude-3-5-sonnet-20241022",
			Usage: parser.Usage{InputTokens: 1000, OutputTokens: 500},
		},
	}

	agg.Add(entry1)
	agg.Add(entry2)

	block := agg.CurrentBillingBlock("")

	if !block.IsActive {
		t.Error("CurrentBillingBlock.IsActive = false, want true")
	}
	if block.TotalTokens != 150 {
		t.Errorf("CurrentBillingBlock.TotalTokens = %d, want 150", block.TotalTokens)
	}
	if block.EntryCount != 1 {
		t.Errorf("CurrentBillingBlock.EntryCount = %d, want 1", block.EntryCount)
	}
}

func TestGetBillingBlockStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		hour     int
		expected int
	}{
		{0, 0},
		{1, 0},
		{4, 0},
		{5, 5},
		{9, 5},
		{10, 10},
		{14, 10},
		{15, 15},
		{19, 15},
		{20, 20},
		{23, 20},
	}

	for _, tc := range tests {
		input := time.Date(2024, 1, 15, tc.hour, 30, 0, 0, time.UTC)
		result := getBillingBlockStart(input)
		if result.Hour() != tc.expected {
			t.Errorf("getBillingBlockStart(%d:30) = %d:00, want %d:00",
				tc.hour, result.Hour(), tc.expected)
		}
	}
}
