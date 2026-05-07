package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/0xmhha/token-monitor/pkg/analysis"
	"github.com/0xmhha/token-monitor/pkg/config"
	"github.com/0xmhha/token-monitor/pkg/discovery"
	"github.com/0xmhha/token-monitor/pkg/logger"
	"github.com/0xmhha/token-monitor/pkg/parser"
	"github.com/0xmhha/token-monitor/pkg/session"
)

// compareOptions holds parsed options for the compare command.
type compareOptions struct {
	identifierA string
	identifierB string
	maxTurns    int
	jsonOutput  bool
}

// runCompare compares two sessions side by side.
func (c *sessionCommand) runCompare(args []string) error {
	opts, err := c.parseCompareOptions(args)
	if err != nil {
		return err
	}

	cfg, log, mgr, err := c.initializeSessionComponents()
	if err != nil {
		return err
	}
	defer func() {
		if mgr != nil {
			mgr.Close() //nolint:errcheck,gosec // best-effort cleanup
		}
	}()

	disc := discovery.New(cfg.ClaudeConfigDirs, log)
	discovered, err := disc.Discover()
	if err != nil {
		return fmt.Errorf("failed to discover sessions: %w", err)
	}

	analysisA, err := c.loadAndAnalyze(cfg, log, mgr, discovered, opts.identifierA)
	if err != nil {
		return fmt.Errorf("session A: %w", err)
	}

	analysisB, err := c.loadAndAnalyze(cfg, log, mgr, discovered, opts.identifierB)
	if err != nil {
		return fmt.Errorf("session B: %w", err)
	}

	if opts.jsonOutput {
		return c.displayCompareJSON(analysisA, analysisB)
	}

	c.displayCompareReport(analysisA, analysisB, opts.maxTurns)
	return nil
}

// parseCompareOptions parses command line flags for compare command.
func (c *sessionCommand) parseCompareOptions(args []string) (*compareOptions, error) {
	fs := flag.NewFlagSet("session compare", flag.ExitOnError)
	maxTurns := fs.Int("turns", 10, "maximum turns to show in turn-by-turn comparison")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if fs.NArg() < 2 {
		return nil, fmt.Errorf("usage: token-monitor session compare <session-a> <session-b> [flags]")
	}

	return &compareOptions{
		identifierA: fs.Arg(0),
		identifierB: fs.Arg(1),
		maxTurns:    *maxTurns,
		jsonOutput:  c.globalOpts.jsonOutput,
	}, nil
}

// loadAndAnalyze finds, parses, and analyzes a session.
func (c *sessionCommand) loadAndAnalyze(
	cfg *config.Config,
	log logger.Logger,
	mgr session.Manager,
	discovered []discovery.SessionFile,
	identifier string,
) (analysis.SessionAnalysis, error) {
	metadata := c.findSessionMetadata(mgr, identifier)
	sessionFile := c.findSessionFile(discovered, identifier, metadata)
	if sessionFile == nil {
		return analysis.SessionAnalysis{}, fmt.Errorf("session file not found: %s", identifier)
	}

	p := parser.New()
	entries, _, err := p.ParseFile(sessionFile.FilePath, 0)
	if err != nil {
		return analysis.SessionAnalysis{}, fmt.Errorf("failed to parse: %w", err)
	}

	if len(entries) == 0 {
		return analysis.SessionAnalysis{}, fmt.Errorf("no usage data in session: %s", identifier)
	}

	displayName := identifier
	if metadata != nil && metadata.Name != "" {
		displayName = metadata.Name
	}

	_ = cfg
	_ = log

	return analysis.Analyze(sessionFile.SessionID, displayName, sessionFile.ProjectPath, entries), nil
}

// displayCompareJSON outputs comparison as JSON.
func (c *sessionCommand) displayCompareJSON(a, b analysis.SessionAnalysis) error {
	output := map[string]any{
		"session_a": buildJSONAnalysis(a),
		"session_b": buildJSONAnalysis(b),
		"diff": map[string]any{
			"total_tokens":   a.TotalTokens - b.TotalTokens,
			"input_tokens":   a.InputTokens - b.InputTokens,
			"output_tokens":  a.OutputTokens - b.OutputTokens,
			"cache_creation": a.CacheCreation - b.CacheCreation,
			"cache_read":     a.CacheRead - b.CacheRead,
			"entry_count":    a.EntryCount - b.EntryCount,
			"cost_usd":       a.CostUSD - b.CostUSD,
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func buildJSONAnalysis(a analysis.SessionAnalysis) map[string]any {
	return map[string]any{
		"session_id":     a.SessionID,
		"label":          a.Label,
		"project":        a.Project,
		"entry_count":    a.EntryCount,
		"duration":       a.Duration.String(),
		"input_tokens":   a.InputTokens,
		"output_tokens":  a.OutputTokens,
		"cache_creation": a.CacheCreation,
		"cache_read":     a.CacheRead,
		"total_tokens":   a.TotalTokens,
		"real_input":     a.RealInput,
		"cache_hit_rate": a.CacheHitRate,
		"cost_usd":       a.CostUSD,
		"tool_usage":     a.ToolUsage,
		"models":         a.Models,
	}
}

// ──────────────────────────────────────────────────────────────────────
// Display helpers
// ──────────────────────────────────────────────────────────────────────

const (
	cmpHeader = "┌──────────────────────────┬──────────────┬──────────────┬──────────────┐"
	cmpSep    = "├──────────────────────────┼──────────────┼──────────────┼──────────────┤"
	cmpFooter = "└──────────────────────────┴──────────────┴──────────────┴──────────────┘"

	cmpHeader2 = "┌──────────────────────────┬──────────────┬──────────────┐"
	cmpSep2    = "├──────────────────────────┼──────────────┼──────────────┤"
	cmpFooter2 = "└──────────────────────────┴──────────────┴──────────────┘"

	cmpTurnHeader = "┌──────┬──────────────┬──────────────┬──────────────┬──────────────────────────┐"
	cmpTurnSep    = "├──────┼──────────────┼──────────────┼──────────────┼──────────────────────────┤"
	cmpTurnFooter = "└──────┴──────────────┴──────────────┴──────────────┴──────────────────────────┘"
)

func (c *sessionCommand) displayCompareReport(a, b analysis.SessionAnalysis, maxTurns int) {
	c.displayCompareHeader(a, b)
	c.displayTokenComparison(a, b)
	c.displayCacheComparison(a, b)
	c.displayCostComparison(a, b)
	c.displayToolComparison(a, b)
	c.displayTurnComparison(a, b, maxTurns)
}

func (c *sessionCommand) displayCompareHeader(a, b analysis.SessionAnalysis) {
	fmt.Println()
	fmt.Println("Session Comparison Report")
	fmt.Println(strings.Repeat("═", 70))

	fmt.Println()
	fmt.Printf("  A: %-20s │ %s\n", a.Label, truncateProjectPath(a.Project, 40))
	fmt.Printf("  B: %-20s │ %s\n", b.Label, truncateProjectPath(b.Project, 40))
	fmt.Println()
}

func (c *sessionCommand) displayTokenComparison(a, b analysis.SessionAnalysis) {
	fmt.Println("Token Usage")
	fmt.Println(cmpHeader)
	fmt.Printf("│ %-24s │ %12s │ %12s │ %12s │\n", "Metric", "Session A", "Session B", "Diff")
	fmt.Println(cmpSep)

	printRow := func(label string, va, vb int) {
		fmt.Printf("│ %-24s │ %12s │ %12s │ %12s │\n",
			label, fmtNum(va), fmtNum(vb), fmtDiff(va-vb))
	}

	printRow("Requests", a.EntryCount, b.EntryCount)
	printRow("Input Tokens", a.InputTokens, b.InputTokens)
	printRow("Output Tokens", a.OutputTokens, b.OutputTokens)
	printRow("Cache Creation", a.CacheCreation, b.CacheCreation)
	printRow("Cache Read", a.CacheRead, b.CacheRead)
	fmt.Println(cmpSep)
	printRow("Total Tokens", a.TotalTokens, b.TotalTokens)
	printRow("Real Input", a.RealInput, b.RealInput)
	fmt.Println(cmpFooter)

	// Duration
	fmt.Printf("\n  Duration:  A = %s  │  B = %s\n\n",
		formatDuration(a.Duration), formatDuration(b.Duration))
}

func (c *sessionCommand) displayCacheComparison(a, b analysis.SessionAnalysis) {
	fmt.Println("Cache Efficiency")
	fmt.Println(cmpHeader2)
	fmt.Printf("│ %-24s │ %12s │ %12s │\n", "Metric", "Session A", "Session B")
	fmt.Println(cmpSep2)

	fmt.Printf("│ %-24s │ %11.1f%% │ %11.1f%% │\n",
		"Cache Hit Rate", a.CacheHitRate, b.CacheHitRate)

	cacheA := a.CacheCreation + a.CacheRead
	cacheB := b.CacheCreation + b.CacheRead
	fmt.Printf("│ %-24s │ %12s │ %12s │\n",
		"Cache Total", fmtNum(cacheA), fmtNum(cacheB))

	// First turn cache_creation (proxy for system prompt size)
	var firstCCA, firstCCB int
	if len(a.Turns) > 0 {
		firstCCA = a.Turns[0].CacheCreation
	}
	if len(b.Turns) > 0 {
		firstCCB = b.Turns[0].CacheCreation
	}
	fmt.Printf("│ %-24s │ %12s │ %12s │\n",
		"1st Turn Cache Create", fmtNum(firstCCA), fmtNum(firstCCB))

	fmt.Println(cmpFooter2)
	fmt.Println()
}

func (c *sessionCommand) displayCostComparison(a, b analysis.SessionAnalysis) {
	inA, outA, cwA, crA := analysis.CostBreakdown(a)
	inB, outB, cwB, crB := analysis.CostBreakdown(b)

	costLabel := "Estimated Cost"
	if a.HasRealCost || b.HasRealCost {
		costLabel = "Cost (from API)"
	}

	fmt.Println(costLabel)
	fmt.Println(cmpHeader)
	fmt.Printf("│ %-24s │ %12s │ %12s │ %12s │\n", "Component", "Session A", "Session B", "Diff")
	fmt.Println(cmpSep)

	printCostRow := func(label string, va, vb float64) {
		fmt.Printf("│ %-24s │ %11s │ %11s │ %11s │\n",
			label, fmtUSD(va), fmtUSD(vb), fmtUSDDiff(va-vb))
	}

	if !a.HasRealCost && !b.HasRealCost {
		printCostRow("Input", inA, inB)
		printCostRow("Output", outA, outB)
		printCostRow("Cache Write", cwA, cwB)
		printCostRow("Cache Read", crA, crB)
		fmt.Println(cmpSep)
	}

	printCostRow("Total", a.CostUSD, b.CostUSD)
	fmt.Println(cmpFooter)

	// Show dominant model for pricing context
	fmt.Printf("  Pricing: A=%s  │  B=%s\n\n", dominantModel(a), dominantModel(b))
}

func (c *sessionCommand) displayToolComparison(a, b analysis.SessionAnalysis) {
	allTools := mergeToolKeys(a.ToolUsage, b.ToolUsage)
	if len(allTools) == 0 {
		return
	}

	fmt.Println("Tool Usage")
	fmt.Println(cmpHeader)
	fmt.Printf("│ %-24s │ %12s │ %12s │ %12s │\n", "Tool", "Session A", "Session B", "Diff")
	fmt.Println(cmpSep)

	for _, tool := range allTools {
		va := a.ToolUsage[tool]
		vb := b.ToolUsage[tool]
		fmt.Printf("│ %-24s │ %12s │ %12s │ %12s │\n",
			truncStr(tool, 24), fmtNum(va), fmtNum(vb), fmtDiff(va-vb))
	}

	totalA, totalB := sumMap(a.ToolUsage), sumMap(b.ToolUsage)
	fmt.Println(cmpSep)
	fmt.Printf("│ %-24s │ %12s │ %12s │ %12s │\n",
		"Total", fmtNum(totalA), fmtNum(totalB), fmtDiff(totalA-totalB))
	fmt.Println(cmpFooter)
	fmt.Println()
}

func (c *sessionCommand) displayTurnComparison(a, b analysis.SessionAnalysis, maxTurns int) {
	maxLen := max(len(a.Turns), len(b.Turns))
	if maxLen == 0 {
		return
	}

	shown := min(maxTurns, maxLen)

	fmt.Printf("Turn-by-Turn (first %d of %d)\n", shown, maxLen)
	fmt.Println(cmpTurnHeader)
	fmt.Printf("│ %4s │ %12s │ %12s │ %12s │ %-24s │\n",
		"Turn", "A Tokens", "B Tokens", "Diff", "A Tools")
	fmt.Println(cmpTurnSep)

	for i := 0; i < shown; i++ {
		var va, vb int
		var toolDesc string

		if i < len(a.Turns) {
			va = a.Turns[i].TotalTokens
			if len(a.Turns[i].Tools) > 0 {
				toolDesc = truncStr(strings.Join(a.Turns[i].Tools, ","), 24)
			}
		}
		if i < len(b.Turns) {
			vb = b.Turns[i].TotalTokens
		}

		fmt.Printf("│ %4d │ %12s │ %12s │ %12s │ %-24s │\n",
			i+1, fmtNum(va), fmtNum(vb), fmtDiff(va-vb), toolDesc)
	}

	fmt.Println(cmpTurnFooter)

	if maxLen > shown {
		fmt.Printf("  ... %d more turns not shown (use -turns %d to see all)\n", maxLen-shown, maxLen)
	}
	fmt.Println()
}

// ──────────────────────────────────────────────────────────────────────
// Formatting utilities
// ──────────────────────────────────────────────────────────────────────

// fmtNum formats an integer with comma separators.
func fmtNum(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		if negative {
			return "-" + s
		}
		return s
	}

	result := make([]byte, 0, len(s)+len(s)/3)
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(ch))
	}

	if negative {
		return "-" + string(result)
	}
	return string(result)
}

// fmtDiff formats a difference with + or - sign and commas.
func fmtDiff(n int) string {
	if n == 0 {
		return "0"
	}
	if n > 0 {
		return "+" + fmtNum(n)
	}
	return fmtNum(n)
}

// fmtUSD formats a dollar amount.
func fmtUSD(v float64) string {
	return fmt.Sprintf("$%.4f", v)
}

// fmtUSDDiff formats a dollar difference with sign.
func fmtUSDDiff(v float64) string {
	if v >= 0 {
		return fmt.Sprintf("+$%.4f", v)
	}
	return fmt.Sprintf("-$%.4f", -v)
}

// truncStr truncates a string to maxLen characters.
func truncStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// dominantModel returns the most-used model name in a session.
func dominantModel(a analysis.SessionAnalysis) string {
	var best string
	var bestCount int
	for m, count := range a.Models {
		if count > bestCount {
			best = m
			bestCount = count
		}
	}
	if len(best) > 30 {
		best = best[:27] + "..."
	}
	return best
}

// mergeToolKeys returns a sorted union of tool names from both maps.
func mergeToolKeys(a, b map[string]int) []string {
	seen := make(map[string]bool)
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}

	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}

	// Sort by total usage descending
	sort.Slice(keys, func(i, j int) bool {
		totalI := a[keys[i]] + b[keys[i]]
		totalJ := a[keys[j]] + b[keys[j]]
		return totalI > totalJ
	})

	return keys
}

// sumMap returns the sum of all values in a map.
func sumMap(m map[string]int) int {
	var total int
	for _, v := range m {
		total += v
	}
	return total
}
