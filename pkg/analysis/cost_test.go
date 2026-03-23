package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookupPricing_ModelFamilies(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"claude-sonnet-4", "sonnet"},
		{"claude-sonnet-4-20250514", "sonnet"},
		{"claude-opus-4-6", "opus"},
		{"claude-opus-4-20250514", "opus"},
		{"claude-haiku-3-5-20250101", "haiku"},
		{"unknown-model", "sonnet"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pricing := LookupPricing(tt.model)

			switch tt.expected {
			case "sonnet":
				assert.Equal(t, 3.0, pricing.InputPerMTok)
				assert.Equal(t, 15.0, pricing.OutputPerMTok)
			case "opus":
				assert.Equal(t, 15.0, pricing.InputPerMTok)
				assert.Equal(t, 75.0, pricing.OutputPerMTok)
			case "haiku":
				assert.Equal(t, 0.80, pricing.InputPerMTok)
				assert.Equal(t, 4.0, pricing.OutputPerMTok)
			}
		})
	}
}

func TestEstimateCost_SingleModel(t *testing.T) {
	a := SessionAnalysis{
		InputTokens:  1_000_000, // 1M input tokens
		OutputTokens: 100_000,   // 100K output tokens
		Models:       map[string]int{"claude-sonnet-4": 5},
	}

	cost := EstimateCost(a)

	// 1M * $3/MTok + 100K * $15/MTok = $3 + $1.5 = $4.5
	assert.InDelta(t, 4.5, cost, 0.01)
}

func TestEstimateCost_MixedModels(t *testing.T) {
	a := SessionAnalysis{
		Models: map[string]int{
			"claude-sonnet-4": 1,
			"claude-opus-4":   1,
		},
		Turns: []TurnData{
			{
				Model:        "claude-sonnet-4",
				InputTokens:  1000,
				OutputTokens: 500,
			},
			{
				Model:        "claude-opus-4",
				InputTokens:  1000,
				OutputTokens: 500,
			},
		},
	}

	cost := EstimateCost(a)

	// Sonnet: 1000*3/1M + 500*15/1M = 0.003 + 0.0075 = 0.0105
	// Opus:   1000*15/1M + 500*75/1M = 0.015 + 0.0375 = 0.0525
	// Total: 0.063
	assert.InDelta(t, 0.063, cost, 0.001)
}

func TestCostBreakdown(t *testing.T) {
	a := SessionAnalysis{
		InputTokens:   100_000,
		OutputTokens:  50_000,
		CacheCreation: 200_000,
		CacheRead:     500_000,
		Models:        map[string]int{"claude-sonnet-4": 1},
	}

	input, output, cacheWrite, cacheRead := CostBreakdown(a)

	assert.InDelta(t, 0.3, input, 0.01)      // 100K * 3 / 1M
	assert.InDelta(t, 0.75, output, 0.01)     // 50K * 15 / 1M
	assert.InDelta(t, 0.75, cacheWrite, 0.01) // 200K * 3.75 / 1M
	assert.InDelta(t, 0.15, cacheRead, 0.01)  // 500K * 0.30 / 1M
}
