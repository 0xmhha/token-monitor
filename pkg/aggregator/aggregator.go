package aggregator

import (
	"sort"
	"sync"

	"github.com/yourusername/token-monitor/pkg/parser"
)

// aggregator implements the Aggregator interface.
type aggregator struct {
	config Config

	mu     sync.RWMutex
	counts []int             // All token counts for percentile calculation
	stats  Statistics        // Overall statistics
	groups map[string]*group // Grouped statistics
}

// group holds statistics for a specific dimension combination.
type group struct {
	counts []int
	stats  Statistics
}

// New creates a new aggregator.
//
// Parameters:
//   - cfg: Aggregator configuration
//
// Returns a configured Aggregator.
func New(cfg Config) Aggregator {
	// Set defaults.
	if cfg.TrackPercentiles && cfg.GroupBy == nil {
		cfg.TrackPercentiles = true
	}

	return &aggregator{
		config: cfg,
		counts: make([]int, 0),
		groups: make(map[string]*group),
	}
}

// Add implements Aggregator.Add.
func (a *aggregator) Add(entry parser.UsageEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()

	total := entry.Message.Usage.TotalTokens()
	input := entry.Message.Usage.InputTokens
	output := entry.Message.Usage.OutputTokens

	// Update overall stats.
	a.updateStats(&a.stats, entry, total, input, output)

	// Track counts for percentiles.
	if a.config.TrackPercentiles {
		a.counts = append(a.counts, total)
	}

	// Update grouped stats.
	if len(a.config.GroupBy) > 0 {
		key := a.dimensionKey(entry)
		g, exists := a.groups[key]
		if !exists {
			g = &group{
				counts: make([]int, 0),
			}
			a.groups[key] = g
		}

		a.updateStats(&g.stats, entry, total, input, output)

		if a.config.TrackPercentiles {
			g.counts = append(g.counts, total)
		}
	}
}

// Stats implements Aggregator.Stats.
func (a *aggregator) Stats() Statistics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := a.stats

	// Calculate percentiles if enabled.
	if a.config.TrackPercentiles && len(a.counts) > 0 {
		counts := make([]int, len(a.counts))
		copy(counts, a.counts)
		sort.Ints(counts)

		stats.P50Tokens = percentile(counts, 50)
		stats.P95Tokens = percentile(counts, 95)
		stats.P99Tokens = percentile(counts, 99)
	}

	return stats
}

// GroupedStats implements Aggregator.GroupedStats.
func (a *aggregator) GroupedStats() map[string]Statistics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]Statistics, len(a.groups))

	for key, g := range a.groups {
		stats := g.stats

		// Calculate percentiles if enabled.
		if a.config.TrackPercentiles && len(g.counts) > 0 {
			counts := make([]int, len(g.counts))
			copy(counts, g.counts)
			sort.Ints(counts)

			stats.P50Tokens = percentile(counts, 50)
			stats.P95Tokens = percentile(counts, 95)
			stats.P99Tokens = percentile(counts, 99)
		}

		result[key] = stats
	}

	return result
}

// TopSessions implements Aggregator.TopSessions.
func (a *aggregator) TopSessions(n int) []SessionStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Collect session stats.
	sessions := make(map[string]*SessionStats)

	for key, g := range a.groups {
		// Only consider groups that include session dimension.
		if !a.hasSessionDimension() {
			continue
		}

		// Parse key to extract session ID and model.
		// Key format depends on dimensions, but session is always present.
		sessionID := a.extractSessionFromKey(key)
		model := a.extractModelFromKey(key)

		if existing, exists := sessions[sessionID]; exists {
			// Merge stats for same session across different dimensions.
			existing.Statistics = a.mergeStats(existing.Statistics, g.stats)
		} else {
			stats := g.stats

			// Calculate percentiles if enabled.
			if a.config.TrackPercentiles && len(g.counts) > 0 {
				counts := make([]int, len(g.counts))
				copy(counts, g.counts)
				sort.Ints(counts)

				stats.P50Tokens = percentile(counts, 50)
				stats.P95Tokens = percentile(counts, 95)
				stats.P99Tokens = percentile(counts, 99)
			}

			sessions[sessionID] = &SessionStats{
				SessionID:  sessionID,
				Model:      model,
				Statistics: stats,
			}
		}
	}

	// Convert to slice and sort by total tokens.
	result := make([]SessionStats, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, *s)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Statistics.TotalTokens > result[j].Statistics.TotalTokens
	})

	// Return top N.
	if n > 0 && n < len(result) {
		result = result[:n]
	}

	return result
}

// Reset implements Aggregator.Reset.
func (a *aggregator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.counts = make([]int, 0)
	a.stats = Statistics{}
	a.groups = make(map[string]*group)
}

// updateStats updates statistics with a new entry.
func (a *aggregator) updateStats(stats *Statistics, entry parser.UsageEntry, total, input, output int) {
	// Update counts.
	stats.Count++
	stats.TotalTokens += total
	stats.InputTokens += input
	stats.OutputTokens += output

	// Update average.
	stats.AvgTokens = float64(stats.TotalTokens) / float64(stats.Count)

	// Update min/max.
	if stats.Count == 1 {
		stats.MinTokens = total
		stats.MaxTokens = total
	} else {
		if total < stats.MinTokens {
			stats.MinTokens = total
		}
		if total > stats.MaxTokens {
			stats.MaxTokens = total
		}
	}

	// Update timestamps.
	if stats.FirstSeen.IsZero() || entry.Timestamp.Before(stats.FirstSeen) {
		stats.FirstSeen = entry.Timestamp
	}
	if stats.LastSeen.IsZero() || entry.Timestamp.After(stats.LastSeen) {
		stats.LastSeen = entry.Timestamp
	}
}

// dimensionKey creates a unique key for the configured dimensions.
func (a *aggregator) dimensionKey(entry parser.UsageEntry) string {
	if len(a.config.GroupBy) == 0 {
		return ""
	}

	key := ""
	for i, dim := range a.config.GroupBy {
		if i > 0 {
			key += "|"
		}

		switch dim {
		case DimModel:
			key += entry.Message.Model
		case DimSession:
			key += entry.SessionID
		case DimDate:
			key += entry.Timestamp.Format("2006-01-02")
		case DimHour:
			key += entry.Timestamp.Format("2006-01-02 15:00")
		}
	}

	return key
}

// hasSessionDimension checks if session is one of the dimensions.
func (a *aggregator) hasSessionDimension() bool {
	for _, dim := range a.config.GroupBy {
		if dim == DimSession {
			return true
		}
	}
	return false
}

// extractSessionFromKey extracts session ID from dimension key.
func (a *aggregator) extractSessionFromKey(key string) string {
	// Find session dimension position.
	sessionIdx := -1
	for i, dim := range a.config.GroupBy {
		if dim == DimSession {
			sessionIdx = i
			break
		}
	}

	if sessionIdx < 0 {
		return ""
	}

	// Parse key components.
	components := splitKey(key)
	if sessionIdx < len(components) {
		return components[sessionIdx]
	}

	return ""
}

// extractModelFromKey extracts model from dimension key.
func (a *aggregator) extractModelFromKey(key string) string {
	// Find model dimension position.
	modelIdx := -1
	for i, dim := range a.config.GroupBy {
		if dim == DimModel {
			modelIdx = i
			break
		}
	}

	if modelIdx < 0 {
		return ""
	}

	// Parse key components.
	components := splitKey(key)
	if modelIdx < len(components) {
		return components[modelIdx]
	}

	return ""
}

// splitKey splits a dimension key into components.
func splitKey(key string) []string {
	components := make([]string, 0)
	current := ""

	for _, ch := range key {
		if ch == '|' {
			components = append(components, current)
			current = ""
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		components = append(components, current)
	}

	return components
}

// mergeStats merges two Statistics structs.
func (a *aggregator) mergeStats(s1, s2 Statistics) Statistics {
	result := Statistics{
		Count:        s1.Count + s2.Count,
		TotalTokens:  s1.TotalTokens + s2.TotalTokens,
		InputTokens:  s1.InputTokens + s2.InputTokens,
		OutputTokens: s1.OutputTokens + s2.OutputTokens,
	}

	result.AvgTokens = float64(result.TotalTokens) / float64(result.Count)

	// Min/max.
	if s1.MinTokens < s2.MinTokens {
		result.MinTokens = s1.MinTokens
	} else {
		result.MinTokens = s2.MinTokens
	}

	if s1.MaxTokens > s2.MaxTokens {
		result.MaxTokens = s1.MaxTokens
	} else {
		result.MaxTokens = s2.MaxTokens
	}

	// Timestamps.
	if s1.FirstSeen.Before(s2.FirstSeen) {
		result.FirstSeen = s1.FirstSeen
	} else {
		result.FirstSeen = s2.FirstSeen
	}

	if s1.LastSeen.After(s2.LastSeen) {
		result.LastSeen = s1.LastSeen
	} else {
		result.LastSeen = s2.LastSeen
	}

	return result
}

// percentile calculates the nth percentile of a sorted slice.
func percentile(sorted []int, p int) int {
	if len(sorted) == 0 {
		return 0
	}

	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}

	// Linear interpolation between closest ranks.
	rank := float64(p) / 100.0 * float64(len(sorted)-1)
	lower := int(rank)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[lower]
	}

	// Interpolate.
	fraction := rank - float64(lower)
	return int(float64(sorted[lower])*(1-fraction) + float64(sorted[upper])*fraction)
}
