package analysis

import "strings"

// Known model pricing (per million tokens, as of 2025).
var knownPricing = map[string]ModelPricing{
	"sonnet": {
		InputPerMTok:      3.0,
		OutputPerMTok:     15.0,
		CacheWritePerMTok: 3.75,
		CacheReadPerMTok:  0.30,
	},
	"opus": {
		InputPerMTok:      15.0,
		OutputPerMTok:     75.0,
		CacheWritePerMTok: 18.75,
		CacheReadPerMTok:  1.50,
	},
	"haiku": {
		InputPerMTok:      0.80,
		OutputPerMTok:     4.0,
		CacheWritePerMTok: 1.0,
		CacheReadPerMTok:  0.08,
	},
}

// LookupPricing finds pricing for a model name by matching known model families.
func LookupPricing(modelName string) ModelPricing {
	lower := strings.ToLower(modelName)

	switch {
	case strings.Contains(lower, "opus"):
		return knownPricing["opus"]
	case strings.Contains(lower, "haiku"):
		return knownPricing["haiku"]
	default:
		return knownPricing["sonnet"]
	}
}

// EstimateCost calculates the estimated API cost for a session.
// Uses per-turn calculation when multiple models are used.
func EstimateCost(a SessionAnalysis) float64 {
	if len(a.Models) <= 1 {
		var modelName string
		for m := range a.Models {
			modelName = m
		}
		pricing := LookupPricing(modelName)
		return tokenCost(a.InputTokens, a.OutputTokens, a.CacheCreation, a.CacheRead, pricing)
	}

	var total float64
	for _, turn := range a.Turns {
		pricing := LookupPricing(turn.Model)
		total += tokenCost(turn.InputTokens, turn.OutputTokens, turn.CacheCreation, turn.CacheRead, pricing)
	}
	return total
}

// CostBreakdown returns per-component cost for display.
func CostBreakdown(a SessionAnalysis) (input, output, cacheWrite, cacheRead float64) {
	if len(a.Models) <= 1 {
		var modelName string
		for m := range a.Models {
			modelName = m
		}
		p := LookupPricing(modelName)
		return float64(a.InputTokens) * p.InputPerMTok / 1_000_000,
			float64(a.OutputTokens) * p.OutputPerMTok / 1_000_000,
			float64(a.CacheCreation) * p.CacheWritePerMTok / 1_000_000,
			float64(a.CacheRead) * p.CacheReadPerMTok / 1_000_000
	}

	for _, turn := range a.Turns {
		p := LookupPricing(turn.Model)
		input += float64(turn.InputTokens) * p.InputPerMTok / 1_000_000
		output += float64(turn.OutputTokens) * p.OutputPerMTok / 1_000_000
		cacheWrite += float64(turn.CacheCreation) * p.CacheWritePerMTok / 1_000_000
		cacheRead += float64(turn.CacheRead) * p.CacheReadPerMTok / 1_000_000
	}
	return
}

func tokenCost(input, output, cacheCreate, cacheRead int, p ModelPricing) float64 {
	return (float64(input)*p.InputPerMTok +
		float64(output)*p.OutputPerMTok +
		float64(cacheCreate)*p.CacheWritePerMTok +
		float64(cacheRead)*p.CacheReadPerMTok) / 1_000_000
}
