package analysis

import (
	"testing"
	"time"

	"github.com/0xmhha/token-monitor/pkg/parser"
	"github.com/stretchr/testify/assert"
)

func TestAnalyze_EmptyEntries(t *testing.T) {
	result := Analyze("test-id", "test", "/project", nil)

	assert.Equal(t, "test-id", result.SessionID)
	assert.Equal(t, "test", result.Label)
	assert.Equal(t, 0, result.EntryCount)
	assert.Empty(t, result.Turns)
}

func TestAnalyze_BasicMetrics(t *testing.T) {
	now := time.Now()
	entries := []parser.UsageEntry{
		{
			Timestamp: now,
			SessionID: "s1",
			Message: parser.Message{
				Model: "claude-sonnet-4",
				Usage: parser.Usage{
					InputTokens:              100,
					OutputTokens:             200,
					CacheCreationInputTokens: 5000,
					CacheReadInputTokens:     3000,
				},
			},
		},
		{
			Timestamp: now.Add(time.Minute),
			SessionID: "s1",
			Message: parser.Message{
				Model: "claude-sonnet-4",
				Usage: parser.Usage{
					InputTokens:              50,
					OutputTokens:             300,
					CacheCreationInputTokens: 1000,
					CacheReadInputTokens:     7000,
				},
			},
		},
	}

	result := Analyze("s1", "test-session", "/proj", entries)

	assert.Equal(t, 2, result.EntryCount)
	assert.Equal(t, 150, result.InputTokens)
	assert.Equal(t, 500, result.OutputTokens)
	assert.Equal(t, 6000, result.CacheCreation)
	assert.Equal(t, 10000, result.CacheRead)
	assert.Equal(t, 16650, result.TotalTokens)
	assert.Equal(t, 16150, result.RealInput) // 150 + 6000 + 10000
	assert.Equal(t, time.Minute, result.Duration)
	assert.Equal(t, 2, result.Models["claude-sonnet-4"])
}

func TestAnalyze_CacheHitRate(t *testing.T) {
	entries := []parser.UsageEntry{
		{
			Timestamp: time.Now(),
			SessionID: "s1",
			Message: parser.Message{
				Model: "claude-sonnet-4",
				Usage: parser.Usage{
					CacheCreationInputTokens: 2000,
					CacheReadInputTokens:     8000,
				},
			},
		},
	}

	result := Analyze("s1", "test", "/proj", entries)

	// 8000 / (2000 + 8000) * 100 = 80.0%
	assert.InDelta(t, 80.0, result.CacheHitRate, 0.1)
}

func TestAnalyze_ToolExtraction(t *testing.T) {
	entries := []parser.UsageEntry{
		{
			Timestamp: time.Now(),
			SessionID: "s1",
			Message: parser.Message{
				Model: "claude-sonnet-4",
				Usage: parser.Usage{OutputTokens: 100},
				Content: []parser.Content{
					{Type: "text", Text: strPtr("hello")},
					{Type: "tool_use", Name: "Read"},
					{Type: "tool_use", Name: "Bash"},
				},
			},
		},
		{
			Timestamp: time.Now(),
			SessionID: "s1",
			Message: parser.Message{
				Model: "claude-sonnet-4",
				Usage: parser.Usage{OutputTokens: 50},
				Content: []parser.Content{
					{Type: "tool_use", Name: "Read"},
				},
			},
		},
	}

	result := Analyze("s1", "test", "/proj", entries)

	assert.Equal(t, 2, result.ToolUsage["Read"])
	assert.Equal(t, 1, result.ToolUsage["Bash"])
	assert.Equal(t, []string{"Read", "Bash"}, result.Turns[0].Tools)
	assert.Equal(t, []string{"Read"}, result.Turns[1].Tools)
}

func TestAnalyze_RealCostUsed(t *testing.T) {
	cost1 := 0.05
	cost2 := 0.03
	entries := []parser.UsageEntry{
		{
			Timestamp: time.Now(),
			SessionID: "s1",
			Message:   parser.Message{Model: "claude-sonnet-4", Usage: parser.Usage{OutputTokens: 100}},
			CostUSD:   &cost1,
		},
		{
			Timestamp: time.Now(),
			SessionID: "s1",
			Message:   parser.Message{Model: "claude-sonnet-4", Usage: parser.Usage{OutputTokens: 50}},
			CostUSD:   &cost2,
		},
	}

	result := Analyze("s1", "test", "/proj", entries)

	assert.True(t, result.HasRealCost)
	assert.InDelta(t, 0.08, result.CostUSD, 0.001)
}

func TestAnalyze_EstimatedCostFallback(t *testing.T) {
	entries := []parser.UsageEntry{
		{
			Timestamp: time.Now(),
			SessionID: "s1",
			Message: parser.Message{
				Model: "claude-sonnet-4",
				Usage: parser.Usage{
					InputTokens:  1000,
					OutputTokens: 500,
				},
			},
		},
	}

	result := Analyze("s1", "test", "/proj", entries)

	assert.False(t, result.HasRealCost)
	assert.Greater(t, result.CostUSD, 0.0)
}

func strPtr(s string) *string {
	return &s
}
