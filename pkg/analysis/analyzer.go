package analysis

import (
	"github.com/0xmhha/token-monitor/pkg/parser"
)

// Analyze performs comprehensive analysis on a session's parsed entries.
// It calculates per-turn breakdowns, tool usage, cache efficiency, and cost.
func Analyze(sessionID, label, project string, entries []parser.UsageEntry) SessionAnalysis {
	result := SessionAnalysis{
		SessionID: sessionID,
		Label:     label,
		Project:   project,
		ToolUsage: make(map[string]int),
		Models:    make(map[string]int),
		Turns:     make([]TurnData, 0, len(entries)),
	}

	if len(entries) == 0 {
		return result
	}

	result.EntryCount = len(entries)
	result.FirstSeen = entries[0].Timestamp
	result.LastSeen = entries[len(entries)-1].Timestamp
	result.Duration = result.LastSeen.Sub(result.FirstSeen)

	var realCostTotal float64
	hasAnyCost := false

	for i, entry := range entries {
		usage := entry.Message.Usage
		realInput := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens

		result.InputTokens += usage.InputTokens
		result.OutputTokens += usage.OutputTokens
		result.CacheCreation += usage.CacheCreationInputTokens
		result.CacheRead += usage.CacheReadInputTokens
		result.TotalTokens += usage.TotalTokens()
		result.RealInput += realInput

		result.Models[entry.Message.Model]++

		// Extract tool names from content blocks
		tools := extractTools(entry.Message.Content)
		for _, t := range tools {
			result.ToolUsage[t]++
		}

		// Track real cost if available
		if entry.CostUSD != nil {
			realCostTotal += *entry.CostUSD
			hasAnyCost = true
		}

		result.Turns = append(result.Turns, TurnData{
			Turn:          i + 1,
			Timestamp:     entry.Timestamp,
			Model:         entry.Message.Model,
			InputTokens:   usage.InputTokens,
			OutputTokens:  usage.OutputTokens,
			CacheCreation: usage.CacheCreationInputTokens,
			CacheRead:     usage.CacheReadInputTokens,
			TotalTokens:   usage.TotalTokens(),
			RealInput:     realInput,
			Tools:         tools,
			CostUSD:       entry.CostUSD,
		})
	}

	// Cache hit rate
	cacheTotal := result.CacheCreation + result.CacheRead
	if cacheTotal > 0 {
		result.CacheHitRate = float64(result.CacheRead) / float64(cacheTotal) * 100
	}

	// Use real cost if available, otherwise estimate
	if hasAnyCost {
		result.CostUSD = realCostTotal
		result.HasRealCost = true
	} else {
		result.CostUSD = EstimateCost(result)
	}

	return result
}

// extractTools returns tool names from content blocks.
func extractTools(contents []parser.Content) []string {
	var tools []string
	for _, c := range contents {
		if c.Type == "tool_use" && c.Name != "" {
			tools = append(tools, c.Name)
		}
	}
	return tools
}
