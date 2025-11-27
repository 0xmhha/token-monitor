package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
	"github.com/0xmhha/token-monitor/pkg/config"
	"github.com/0xmhha/token-monitor/pkg/discovery"
	"github.com/0xmhha/token-monitor/pkg/logger"
	"github.com/0xmhha/token-monitor/pkg/parser"
	"github.com/0xmhha/token-monitor/pkg/session"
)

// sessionCommand handles session management subcommands.
type sessionCommand struct {
	configPath string
	globalOpts globalOptions
}

// Execute runs the session command.
func (c *sessionCommand) Execute(args []string) error {
	if len(args) == 0 {
		return c.showHelp()
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "name":
		return c.runName(subargs)
	case "list":
		return c.runList(subargs)
	case "show":
		return c.runShow(subargs)
	case "delete":
		return c.runDelete(subargs)
	case "export":
		return c.runExport(subargs)
	case "help":
		return c.showHelp()
	default:
		return fmt.Errorf("unknown session subcommand: %s", subcommand)
	}
}

// nameArgs holds parsed arguments for the name command.
type nameArgs struct {
	uuid string
	name string
}

// runName assigns a name to a session.
func (c *sessionCommand) runName(args []string) error {
	nameArgs, err := c.parseNameArgs(args)
	if err != nil {
		return err
	}

	cfg, log, mgr, err := c.initializeSessionComponents()
	if err != nil {
		return err
	}
	defer func() {
		if mgr != nil {
			_ = mgr.Close() //nolint:errcheck // best effort cleanup
		}
	}()

	existing, err := mgr.GetByUUID(nameArgs.uuid)
	if err == session.ErrSessionNotFound {
		return c.createNewSession(cfg, log, mgr, nameArgs)
	}
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	return c.updateExistingSession(mgr, nameArgs, existing.Name)
}

// parseNameArgs parses and validates name command arguments.
func (c *sessionCommand) parseNameArgs(args []string) (*nameArgs, error) {
	fs := flag.NewFlagSet("session name", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if fs.NArg() < 2 {
		return nil, fmt.Errorf("usage: token-monitor session name <uuid> <name>")
	}

	name := fs.Arg(1)
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}

	return &nameArgs{uuid: fs.Arg(0), name: name}, nil
}

// createNewSession creates a new session with the given name.
func (c *sessionCommand) createNewSession(
	cfg *config.Config,
	log logger.Logger,
	mgr session.Manager,
	args *nameArgs,
) error {
	projectPath, err := c.findProjectPath(cfg, log, args.uuid)
	if err != nil {
		return err
	}

	metadata := &session.Metadata{
		UUID:        args.uuid,
		Name:        args.name,
		ProjectPath: projectPath,
	}

	if err := mgr.Create(metadata); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	fmt.Printf("Created session '%s' with name '%s'\n", args.uuid[:8], args.name)
	return nil
}

// findProjectPath discovers the project path for a session UUID.
func (c *sessionCommand) findProjectPath(cfg *config.Config, log logger.Logger, uuid string) (string, error) {
	disc := discovery.New(cfg.ClaudeConfigDirs, log)
	sessions, err := disc.Discover()
	if err != nil {
		return "", fmt.Errorf("failed to discover sessions: %w", err)
	}

	for _, s := range sessions {
		if s.SessionID == uuid {
			return s.ProjectPath, nil
		}
	}

	return "", fmt.Errorf("session %s not found in discovered sessions", uuid)
}

// updateExistingSession updates the name of an existing session.
func (c *sessionCommand) updateExistingSession(mgr session.Manager, args *nameArgs, oldName string) error {
	if err := mgr.SetName(args.uuid, args.name); err != nil {
		if err == session.ErrNameConflict {
			return fmt.Errorf("name '%s' is already used by another session", args.name)
		}
		return fmt.Errorf("failed to set name: %w", err)
	}

	c.printNameUpdateResult(args.uuid, oldName, args.name)
	return nil
}

// printNameUpdateResult outputs the appropriate message for name updates.
func (c *sessionCommand) printNameUpdateResult(uuid, oldName, newName string) {
	shortUUID := uuid[:8]
	switch {
	case oldName == newName:
		fmt.Printf("Session '%s' already has name '%s'\n", shortUUID, newName)
	case oldName == "":
		fmt.Printf("Set name '%s' for session '%s'\n", newName, shortUUID)
	default:
		fmt.Printf("Renamed session '%s' from '%s' to '%s'\n", shortUUID, oldName, newName)
	}
}

// displaySession represents a session for display purposes.
type displaySession struct {
	UUID        string
	Name        string
	ProjectPath string
	UpdatedAt   time.Time
	TotalTokens int
	EntryCount  int
	FilePath    string
}

// listOptions holds parsed options for the list command.
type listOptions struct {
	sortBy      string
	showAll     bool
	project     string
	from        string
	to          string
	minTokens   int
	showTokens  bool
}

// runList lists all sessions with metadata.
func (c *sessionCommand) runList(args []string) error {
	opts, err := c.parseListOptions(args)
	if err != nil {
		return err
	}

	cfg, log, mgr, err := c.initializeSessionComponents()
	if err != nil {
		return err
	}
	defer func() {
		if mgr != nil {
			_ = mgr.Close() //nolint:errcheck // best effort cleanup
		}
	}()

	sessions, err := c.collectSessions(cfg, log, mgr, opts.showAll)
	if err != nil {
		return err
	}

	// Apply filters.
	sessions = c.filterSessions(sessions, opts)

	if len(sessions) == 0 {
		return c.displayEmptyListMessage(opts.showAll)
	}

	c.sortSessions(sessions, opts.sortBy)
	return c.displaySessionListWithOptions(sessions, opts)
}

// parseListOptions parses command line flags for list command.
func (c *sessionCommand) parseListOptions(args []string) (*listOptions, error) {
	fs := flag.NewFlagSet("session list", flag.ExitOnError)
	sortBy := fs.String("sort", "name", "sort by: name, date, uuid, tokens")
	showAll := fs.Bool("all", false, "show all sessions including unnamed")
	project := fs.String("project", "", "filter by project path (substring match)")
	from := fs.String("from", "", "filter sessions updated after date (YYYY-MM-DD)")
	to := fs.String("to", "", "filter sessions updated before date (YYYY-MM-DD)")
	minTokens := fs.Int("min-tokens", 0, "filter sessions with at least N tokens")
	showTokens := fs.Bool("tokens", false, "show token counts in output")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	return &listOptions{
		sortBy:     *sortBy,
		showAll:    *showAll,
		project:    *project,
		from:       *from,
		to:         *to,
		minTokens:  *minTokens,
		showTokens: *showTokens || *minTokens > 0 || *sortBy == "tokens",
	}, nil
}

// initializeSessionComponents sets up common session command dependencies.
func (c *sessionCommand) initializeSessionComponents() (*config.Config, logger.Logger, session.Manager, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	log := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	mgr, err := session.New(session.Config{
		DBPath: cfg.Storage.DBPath,
	}, log)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize session manager: %w", err)
	}

	return cfg, log, mgr, nil
}

// collectSessions gathers and combines discovered and named sessions.
func (c *sessionCommand) collectSessions(
	cfg *config.Config,
	log logger.Logger,
	mgr session.Manager,
	showAll bool,
) ([]displaySession, error) {
	disc := discovery.New(cfg.ClaudeConfigDirs, log)
	discoveredSessions, err := disc.Discover()
	if err != nil {
		return nil, fmt.Errorf("failed to discover sessions: %w", err)
	}

	namedSessions, err := mgr.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	namedMap := buildNamedSessionMap(namedSessions)
	sessions := combineSessionsForDisplay(discoveredSessions, namedMap, showAll)

	// Enrich sessions with token counts.
	c.enrichSessionsWithTokenCounts(sessions)

	return sessions, nil
}

// enrichSessionsWithTokenCounts adds token usage data to sessions.
func (c *sessionCommand) enrichSessionsWithTokenCounts(sessions []displaySession) {
	p := parser.New()

	for i := range sessions {
		if sessions[i].FilePath == "" {
			continue
		}

		entries, _, err := p.ParseFile(sessions[i].FilePath, 0)
		if err != nil {
			continue
		}

		var totalTokens int
		for _, entry := range entries {
			totalTokens += entry.Message.Usage.TotalTokens()
		}

		sessions[i].TotalTokens = totalTokens
		sessions[i].EntryCount = len(entries)
	}
}

// filterSessions applies filters to the session list.
func (c *sessionCommand) filterSessions(sessions []displaySession, opts *listOptions) []displaySession {
	if !c.hasActiveFilters(opts) {
		return sessions
	}

	fromDate, toDate := c.parseDateFilters(opts)
	filtered := make([]displaySession, 0, len(sessions))

	for _, s := range sessions {
		if c.sessionMatchesFilters(s, opts, fromDate, toDate) {
			filtered = append(filtered, s)
		}
	}

	return filtered
}

// hasActiveFilters checks if any filters are enabled.
func (c *sessionCommand) hasActiveFilters(opts *listOptions) bool {
	return opts.project != "" || opts.from != "" || opts.to != "" || opts.minTokens > 0
}

// parseDateFilters parses from and to date strings.
func (c *sessionCommand) parseDateFilters(opts *listOptions) (time.Time, time.Time) {
	var fromDate, toDate time.Time

	if opts.from != "" {
		if t, err := time.Parse("2006-01-02", opts.from); err == nil {
			fromDate = t
		}
	}
	if opts.to != "" {
		if t, err := time.Parse("2006-01-02", opts.to); err == nil {
			toDate = t.Add(24*time.Hour - time.Second) // End of day
		}
	}

	return fromDate, toDate
}

// sessionMatchesFilters checks if a session passes all filters.
func (c *sessionCommand) sessionMatchesFilters(s displaySession, opts *listOptions, fromDate, toDate time.Time) bool {
	// Filter by project path.
	if opts.project != "" && !strings.Contains(strings.ToLower(s.ProjectPath), strings.ToLower(opts.project)) {
		return false
	}

	// Filter by date range (from).
	if !fromDate.IsZero() && !s.UpdatedAt.IsZero() && s.UpdatedAt.Before(fromDate) {
		return false
	}

	// Filter by date range (to).
	if !toDate.IsZero() && !s.UpdatedAt.IsZero() && s.UpdatedAt.After(toDate) {
		return false
	}

	// Filter by minimum tokens.
	if opts.minTokens > 0 && s.TotalTokens < opts.minTokens {
		return false
	}

	return true
}

// buildNamedSessionMap creates a UUID to metadata map.
func buildNamedSessionMap(sessions []*session.Metadata) map[string]*session.Metadata {
	namedMap := make(map[string]*session.Metadata)
	for _, s := range sessions {
		namedMap[s.UUID] = s
	}
	return namedMap
}

// combineSessionsForDisplay merges discovered sessions with metadata.
func combineSessionsForDisplay(
	discovered []discovery.SessionFile,
	namedMap map[string]*session.Metadata,
	showAll bool,
) []displaySession {
	var sessions []displaySession

	for _, ds := range discovered {
		if named, ok := namedMap[ds.SessionID]; ok {
			sessions = append(sessions, displaySession{
				UUID:        ds.SessionID,
				Name:        named.Name,
				ProjectPath: ds.ProjectPath,
				UpdatedAt:   named.UpdatedAt,
				FilePath:    ds.FilePath,
			})
		} else if showAll {
			sessions = append(sessions, displaySession{
				UUID:        ds.SessionID,
				Name:        "(unnamed)",
				ProjectPath: ds.ProjectPath,
				UpdatedAt:   time.Time{},
				FilePath:    ds.FilePath,
			})
		}
	}

	return sessions
}

// displayEmptyListMessage shows appropriate message when no sessions found.
func (c *sessionCommand) displayEmptyListMessage(showAll bool) error { //nolint:unparam // error return kept for consistency
	if showAll {
		fmt.Println("No sessions found")
	} else {
		fmt.Println("No named sessions found. Use -all to show all sessions.")
	}
	return nil
}

// sortSessions sorts the session list by the specified criteria.
func (c *sessionCommand) sortSessions(sessions []displaySession, sortBy string) {
	switch sortBy {
	case "name":
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].Name < sessions[j].Name
		})
	case "date":
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
		})
	case "uuid":
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].UUID < sessions[j].UUID
		})
	case "tokens":
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].TotalTokens > sessions[j].TotalTokens
		})
	}
}

// displaySessionListWithOptions renders the session list with optional columns.
func (c *sessionCommand) displaySessionListWithOptions(sessions []displaySession, opts *listOptions) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if err := c.writeSessionTableHeaderWithOptions(w, opts); err != nil {
		return err
	}

	for _, s := range sessions {
		if err := c.writeSessionRowWithOptions(w, s, opts); err != nil {
			return err
		}
	}

	if err := w.Flush(); err != nil {
		return err
	}

	// Show filter summary if filters were applied.
	c.displayFilterSummary(len(sessions), opts)

	return nil
}

// displayFilterSummary shows what filters were applied.
func (c *sessionCommand) displayFilterSummary(count int, opts *listOptions) {
	var filters []string

	if opts.project != "" {
		filters = append(filters, fmt.Sprintf("project contains '%s'", opts.project))
	}
	if opts.from != "" {
		filters = append(filters, fmt.Sprintf("from %s", opts.from))
	}
	if opts.to != "" {
		filters = append(filters, fmt.Sprintf("to %s", opts.to))
	}
	if opts.minTokens > 0 {
		filters = append(filters, fmt.Sprintf("min %d tokens", opts.minTokens))
	}

	if len(filters) > 0 {
		fmt.Printf("\nFilters: %s\n", strings.Join(filters, ", "))
	}
	fmt.Printf("Total: %d session(s)\n", count)
}

// writeSessionTableHeaderWithOptions writes the table header with optional columns.
func (c *sessionCommand) writeSessionTableHeaderWithOptions(w *tabwriter.Writer, opts *listOptions) error {
	header := "NAME\tUUID\tPROJECT\tLAST UPDATED"
	separator := "----\t----\t-------\t------------"

	if opts.showTokens {
		header += "\tTOKENS\tREQUESTS"
		separator += "\t------\t--------"
	}

	if _, err := fmt.Fprintln(w, header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := fmt.Fprintln(w, separator); err != nil {
		return fmt.Errorf("failed to write header separator: %w", err)
	}
	return nil
}

// writeSessionRowWithOptions writes a single session row with optional columns.
func (c *sessionCommand) writeSessionRowWithOptions(w *tabwriter.Writer, s displaySession, opts *listOptions) error {
	shortUUID := s.UUID[:8] + "..."
	projectName := truncateProjectPath(s.ProjectPath, 30)
	updated := formatUpdateTime(s.UpdatedAt)

	row := fmt.Sprintf("%s\t%s\t%s\t%s", s.Name, shortUUID, projectName, updated)

	if opts.showTokens {
		row += fmt.Sprintf("\t%d\t%d", s.TotalTokens, s.EntryCount)
	}

	if _, err := fmt.Fprintln(w, row); err != nil {
		return fmt.Errorf("failed to write session: %w", err)
	}
	return nil
}

// truncateProjectPath shortens long project paths.
func truncateProjectPath(path string, maxLen int) string {
	if len(path) > maxLen {
		return "..." + path[len(path)-(maxLen-3):]
	}
	return path
}

// formatUpdateTime formats the update time for display.
func formatUpdateTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

// showOptions holds parsed options for the show command.
type showOptions struct {
	identifier string
	detailed   bool
}

// runShow displays detailed session information.
func (c *sessionCommand) runShow(args []string) error {
	opts, err := c.parseShowOptions(args)
	if err != nil {
		return err
	}

	cfg, log, mgr, err := c.initializeSessionComponents()
	if err != nil {
		return err
	}
	defer func() {
		if mgr != nil {
			_ = mgr.Close() //nolint:errcheck // best effort cleanup
		}
	}()

	metadata, sessionFile, err := c.findSessionForShow(cfg, log, mgr, opts.identifier)
	if err != nil {
		return err
	}

	c.displaySessionMetadata(metadata)

	if sessionFile != nil {
		if err := c.displaySessionStats(sessionFile, metadata.UUID); err != nil {
			log.Warn("failed to display session stats", "error", err)
		}
	}

	return nil
}

// parseShowOptions parses command line flags for show command.
func (c *sessionCommand) parseShowOptions(args []string) (*showOptions, error) {
	fs := flag.NewFlagSet("session show", flag.ExitOnError)
	detailed := fs.Bool("detailed", true, "show detailed statistics")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if fs.NArg() < 1 {
		return nil, fmt.Errorf("usage: token-monitor session show <name|uuid>")
	}

	return &showOptions{identifier: fs.Arg(0), detailed: *detailed}, nil
}

// findSessionForShow finds session metadata and file for the show command.
func (c *sessionCommand) findSessionForShow(
	cfg *config.Config,
	log logger.Logger,
	mgr session.Manager,
	identifier string,
) (*session.Metadata, *discovery.SessionFile, error) {
	// Try to find session by name first, then by UUID.
	metadata, err := mgr.GetByName(identifier)
	if err != nil {
		if err == session.ErrSessionNotFound {
			metadata, err = mgr.GetByUUID(identifier)
			if err != nil {
				return nil, nil, fmt.Errorf("session not found: %s", identifier)
			}
		} else {
			return nil, nil, fmt.Errorf("failed to get session: %w", err)
		}
	}

	// Find the session file.
	disc := discovery.New(cfg.ClaudeConfigDirs, log)
	discoveredSessions, err := disc.Discover()
	if err != nil {
		return metadata, nil, nil // Return metadata even if discovery fails
	}

	for i := range discoveredSessions {
		if discoveredSessions[i].SessionID == metadata.UUID {
			return metadata, &discoveredSessions[i], nil
		}
	}

	return metadata, nil, nil
}

// displaySessionMetadata shows basic session metadata.
func (c *sessionCommand) displaySessionMetadata(metadata *session.Metadata) {
	fmt.Println("📋 Session Details")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("UUID:        %s\n", metadata.UUID)
	fmt.Printf("Name:        %s\n", metadata.Name)
	fmt.Printf("Project:     %s\n", metadata.ProjectPath)
	fmt.Printf("Created:     %s\n", metadata.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:     %s\n", metadata.UpdatedAt.Format("2006-01-02 15:04:05"))

	if len(metadata.Tags) > 0 {
		fmt.Printf("Tags:        %s\n", strings.Join(metadata.Tags, ", "))
	}

	if metadata.Description != "" {
		fmt.Printf("Description: %s\n", metadata.Description)
	}
}

// displaySessionStats shows token statistics, billing blocks, and activity timeline.
func (c *sessionCommand) displaySessionStats(sessionFile *discovery.SessionFile, sessionID string) error {
	// Parse the session file.
	p := parser.New()
	entries, _, err := p.ParseFile(sessionFile.FilePath, 0)
	if err != nil {
		return fmt.Errorf("failed to parse session file: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("\nNo usage data found for this session.")
		return nil
	}

	// Create aggregator and add entries.
	agg := aggregator.New(aggregator.Config{
		TrackPercentiles: true,
	})

	for _, entry := range entries {
		entry.SessionID = sessionID
		agg.Add(entry)
	}

	c.displayTokenBreakdown(agg.Stats(), entries)
	c.displayBillingBlocks(agg.BillingBlocks(sessionID))
	c.displayActivityTimeline(entries)

	return nil
}

// displayTokenBreakdown shows token usage breakdown by type.
func (c *sessionCommand) displayTokenBreakdown(stats aggregator.Statistics, entries []parser.UsageEntry) {
	// Calculate cache token totals.
	var cacheCreation, cacheRead int
	for _, entry := range entries {
		cacheCreation += entry.Message.Usage.CacheCreationInputTokens
		cacheRead += entry.Message.Usage.CacheReadInputTokens
	}

	fmt.Println()
	fmt.Println("📊 Token Breakdown")
	fmt.Println("┌────────────────────────┬──────────────┬─────────┐")
	fmt.Println("│ Token Type             │        Count │   Share │")
	fmt.Println("├────────────────────────┼──────────────┼─────────┤")

	total := stats.TotalTokens
	if total == 0 {
		total = 1 // Avoid division by zero
	}

	fmt.Printf("│ Input Tokens           │ %12d │ %6.1f%% │\n",
		stats.InputTokens, float64(stats.InputTokens)*100/float64(total))
	fmt.Printf("│ Output Tokens          │ %12d │ %6.1f%% │\n",
		stats.OutputTokens, float64(stats.OutputTokens)*100/float64(total))
	fmt.Printf("│ Cache Creation Tokens  │ %12d │ %6.1f%% │\n",
		cacheCreation, float64(cacheCreation)*100/float64(total))
	fmt.Printf("│ Cache Read Tokens      │ %12d │ %6.1f%% │\n",
		cacheRead, float64(cacheRead)*100/float64(total))
	fmt.Println("├────────────────────────┼──────────────┼─────────┤")
	fmt.Printf("│ Total Tokens           │ %12d │ %6.1f%% │\n", stats.TotalTokens, 100.0)
	fmt.Println("└────────────────────────┴──────────────┴─────────┘")

	// Statistics summary.
	fmt.Println()
	fmt.Println("📈 Statistics")
	fmt.Println("┌────────────────────────┬──────────────┐")
	fmt.Println("│ Metric                 │        Value │")
	fmt.Println("├────────────────────────┼──────────────┤")
	fmt.Printf("│ Total Requests         │ %12d │\n", stats.Count)
	fmt.Printf("│ Average Tokens/Request │ %12.0f │\n", stats.AvgTokens)
	fmt.Printf("│ Min Tokens             │ %12d │\n", stats.MinTokens)
	fmt.Printf("│ Max Tokens             │ %12d │\n", stats.MaxTokens)
	if stats.P50Tokens > 0 {
		fmt.Printf("│ P50 Tokens             │ %12d │\n", stats.P50Tokens)
		fmt.Printf("│ P95 Tokens             │ %12d │\n", stats.P95Tokens)
		fmt.Printf("│ P99 Tokens             │ %12d │\n", stats.P99Tokens)
	}
	fmt.Println("└────────────────────────┴──────────────┘")
}

// displayBillingBlocks shows the billing blocks timeline.
func (c *sessionCommand) displayBillingBlocks(blocks []aggregator.BillingBlock) {
	if len(blocks) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("⏰ Billing Blocks (5-hour UTC windows)")
	fmt.Println("┌─────────────────────────────────┬──────────────┬──────────┬────────┐")
	fmt.Println("│ Time Window (UTC)               │       Tokens │ Requests │ Status │")
	fmt.Println("├─────────────────────────────────┼──────────────┼──────────┼────────┤")

	// Show up to 10 most recent blocks.
	maxBlocks := 10
	if len(blocks) < maxBlocks {
		maxBlocks = len(blocks)
	}

	for i := 0; i < maxBlocks; i++ {
		block := blocks[i]
		status := "  past"
		if block.IsActive {
			status = "🔴 now"
		}

		timeWindow := fmt.Sprintf("%s - %s",
			block.StartTime.Format("2006-01-02 15:04"),
			block.EndTime.Format("15:04"))

		fmt.Printf("│ %-31s │ %12d │ %8d │ %s │\n",
			timeWindow, block.TotalTokens, block.EntryCount, status)
	}

	fmt.Println("└─────────────────────────────────┴──────────────┴──────────┴────────┘")

	if len(blocks) > maxBlocks {
		fmt.Printf("  ... and %d more billing blocks\n", len(blocks)-maxBlocks)
	}
}

// displayActivityTimeline shows recent activity timestamps.
func (c *sessionCommand) displayActivityTimeline(entries []parser.UsageEntry) {
	if len(entries) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("📅 Activity Timeline")
	fmt.Println("┌─────────────────────┬────────────────────────────────┬────────────┐")
	fmt.Println("│ Timestamp           │ Model                          │     Tokens │")
	fmt.Println("├─────────────────────┼────────────────────────────────┼────────────┤")

	// Show up to 15 most recent entries.
	maxEntries := 15
	startIdx := 0
	if len(entries) > maxEntries {
		startIdx = len(entries) - maxEntries
	}

	for i := startIdx; i < len(entries); i++ {
		entry := entries[i]
		model := entry.Message.Model
		if len(model) > 30 {
			model = model[:27] + "..."
		}

		fmt.Printf("│ %s │ %-30s │ %10d │\n",
			entry.Timestamp.Format("2006-01-02 15:04"),
			model,
			entry.Message.Usage.TotalTokens())
	}

	fmt.Println("└─────────────────────┴────────────────────────────────┴────────────┘")

	if len(entries) > maxEntries {
		fmt.Printf("  Showing last %d of %d entries\n", maxEntries, len(entries))
	}

	// Time span.
	if len(entries) >= 2 {
		first := entries[0].Timestamp
		last := entries[len(entries)-1].Timestamp
		duration := last.Sub(first)
		fmt.Printf("\n  Session span: %s → %s (%s)\n",
			first.Format("2006-01-02 15:04"),
			last.Format("2006-01-02 15:04"),
			formatDuration(duration))
	}
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// runDelete removes session metadata.
func (c *sessionCommand) runDelete(args []string) error {
	fs := flag.NewFlagSet("session delete", flag.ExitOnError)
	force := fs.Bool("force", false, "skip confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: token-monitor session delete <name|uuid>")
	}

	identifier := fs.Arg(0)

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger.
	log := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	// Initialize session manager.
	mgr, err := session.New(session.Config{
		DBPath: cfg.Storage.DBPath,
	}, log)
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}
	defer func() {
		if closeErr := mgr.Close(); closeErr != nil {
			log.Error("failed to close session manager", "error", closeErr)
		}
	}()

	// Try to find session by name first, then by UUID.
	var metadata *session.Metadata

	metadata, err = mgr.GetByName(identifier)
	if err != nil {
		if err == session.ErrSessionNotFound {
			// Try by UUID.
			metadata, err = mgr.GetByUUID(identifier)
			if err != nil {
				return fmt.Errorf("session not found: %s", identifier)
			}
		} else {
			return fmt.Errorf("failed to get session: %w", err)
		}
	}

	// Confirm deletion.
	if !*force {
		fmt.Printf("Delete session '%s' (%s)? [y/N]: ", metadata.Name, metadata.UUID[:8])
		var response string
		if _, scanErr := fmt.Scanln(&response); scanErr != nil {
			return fmt.Errorf("cancelled")
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Delete session.
	if err := mgr.Delete(metadata.UUID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	fmt.Printf("Deleted session '%s' (%s)\n", metadata.Name, metadata.UUID[:8])
	fmt.Println("Note: JSONL data files are preserved.")

	return nil
}

// ExportData represents exported session data.
type ExportData struct {
	SessionID   string        `json:"session_id"`
	Name        string        `json:"name,omitempty"`
	ProjectPath string        `json:"project_path"`
	CreatedAt   time.Time     `json:"created_at,omitempty"`
	UpdatedAt   time.Time     `json:"updated_at,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
	Description string        `json:"description,omitempty"`
	Entries     []ExportEntry `json:"entries"`
	Summary     ExportSummary `json:"summary"`
}

// ExportEntry represents a single usage entry for export.
type ExportEntry struct {
	Timestamp                time.Time `json:"timestamp"`
	Model                    string    `json:"model"`
	InputTokens              int       `json:"input_tokens"`
	OutputTokens             int       `json:"output_tokens"`
	CacheCreationInputTokens int       `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int       `json:"cache_read_input_tokens"`
	TotalTokens              int       `json:"total_tokens"`
	CostUSD                  *float64  `json:"cost_usd,omitempty"`
}

// ExportSummary contains aggregated statistics for the session.
type ExportSummary struct {
	TotalEntries             int     `json:"total_entries"`
	TotalInputTokens         int     `json:"total_input_tokens"`
	TotalOutputTokens        int     `json:"total_output_tokens"`
	TotalCacheCreationTokens int     `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int     `json:"total_cache_read_tokens"`
	TotalTokens              int     `json:"total_tokens"`
	TotalCostUSD             float64 `json:"total_cost_usd,omitempty"`
	FirstEntry               string  `json:"first_entry,omitempty"`
	LastEntry                string  `json:"last_entry,omitempty"`
}

// runExport exports session data to CSV or JSON format.
func (c *sessionCommand) runExport(args []string) error {
	fs := flag.NewFlagSet("session export", flag.ExitOnError)
	format := fs.String("format", "json", "output format: json, csv")
	output := fs.String("output", "", "output file path (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: token-monitor session export <name|uuid> [flags]")
	}

	identifier := fs.Arg(0)

	// Validate format.
	*format = strings.ToLower(*format)
	if *format != "json" && *format != "csv" {
		return fmt.Errorf("invalid format '%s': must be 'json' or 'csv'", *format)
	}

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger.
	log := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	// Find session and parse entries.
	sessionFile, metadata, entries, err := c.findAndParseSession(identifier, cfg, log)
	if err != nil {
		return err
	}

	// Build export data.
	exportData := buildExportData(sessionFile, metadata, entries)

	// Write to output.
	return c.writeExportOutput(*format, *output, exportData, len(entries), log)
}

// findAndParseSession finds a session by identifier and parses its entries.
func (c *sessionCommand) findAndParseSession(
	identifier string,
	cfg *config.Config,
	log logger.Logger,
) (*discovery.SessionFile, *session.Metadata, []parser.UsageEntry, error) {
	// Initialize session manager.
	mgr, err := session.New(session.Config{
		DBPath: cfg.Storage.DBPath,
	}, log)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize session manager: %w", err)
	}
	defer func() {
		if closeErr := mgr.Close(); closeErr != nil {
			log.Error("failed to close session manager", "error", closeErr)
		}
	}()

	// Discover sessions to find the file path.
	disc := discovery.New(cfg.ClaudeConfigDirs, log)
	discoveredSessions, err := disc.Discover()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to discover sessions: %w", err)
	}

	// Try to find session metadata by name or UUID.
	metadata := c.findSessionMetadata(mgr, identifier)

	// Find the session file.
	sessionFile := c.findSessionFile(discoveredSessions, identifier, metadata)
	if sessionFile == nil {
		return nil, nil, nil, fmt.Errorf("session file not found: %s", identifier)
	}

	// Parse the session file.
	p := parser.New()
	entries, _, err := p.ParseFile(sessionFile.FilePath, 0)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return sessionFile, metadata, entries, nil
}

// findSessionMetadata tries to find session metadata by name or UUID.
func (c *sessionCommand) findSessionMetadata(mgr session.Manager, identifier string) *session.Metadata {
	metadata, err := mgr.GetByName(identifier)
	if err == nil {
		return metadata
	}

	if err == session.ErrSessionNotFound {
		metadata, err = mgr.GetByUUID(identifier)
		if err == nil {
			return metadata
		}
	}

	return nil
}

// findSessionFile finds the session file from discovered sessions.
func (c *sessionCommand) findSessionFile(
	sessions []discovery.SessionFile,
	identifier string,
	metadata *session.Metadata,
) *discovery.SessionFile {
	sessionUUID := identifier
	if metadata != nil {
		sessionUUID = metadata.UUID
	}

	for i := range sessions {
		if sessions[i].SessionID == sessionUUID {
			return &sessions[i]
		}
		// Also try matching by partial UUID.
		if strings.HasPrefix(sessions[i].SessionID, identifier) {
			return &sessions[i]
		}
	}

	return nil
}

// writeExportOutput writes export data to the specified output.
func (c *sessionCommand) writeExportOutput(
	format, output string,
	data ExportData,
	entryCount int,
	log logger.Logger,
) error {
	var writer *os.File

	if output != "" {
		// Ensure directory exists.
		dir := filepath.Dir(output)
		if mkdirErr := os.MkdirAll(dir, 0750); mkdirErr != nil {
			return fmt.Errorf("failed to create output directory: %w", mkdirErr)
		}

		// #nosec G304: output path comes from user CLI argument
		var createErr error
		writer, createErr = os.Create(output) //nolint:gosec
		if createErr != nil {
			return fmt.Errorf("failed to create output file: %w", createErr)
		}
		defer func() {
			if closeErr := writer.Close(); closeErr != nil {
				log.Error("failed to close output file", "error", closeErr)
			}
		}()
	} else {
		writer = os.Stdout
	}

	// Write output.
	switch format {
	case "json":
		if err := writeJSON(writer, data); err != nil {
			return fmt.Errorf("failed to write JSON: %w", err)
		}
	case "csv":
		if err := writeCSV(writer, data); err != nil {
			return fmt.Errorf("failed to write CSV: %w", err)
		}
	}

	if output != "" {
		fmt.Printf("Exported %d entries to %s\n", entryCount, output)
	}

	return nil
}

// buildExportData creates ExportData from session information and entries.
func buildExportData(sessionFile *discovery.SessionFile, metadata *session.Metadata, entries []parser.UsageEntry) ExportData {
	data := ExportData{
		SessionID:   sessionFile.SessionID,
		ProjectPath: sessionFile.ProjectPath,
		Entries:     make([]ExportEntry, 0, len(entries)),
	}

	if metadata != nil {
		data.Name = metadata.Name
		data.CreatedAt = metadata.CreatedAt
		data.UpdatedAt = metadata.UpdatedAt
		data.Tags = metadata.Tags
		data.Description = metadata.Description
	}

	var summary ExportSummary
	var totalCost float64

	for _, entry := range entries {
		exportEntry := ExportEntry{
			Timestamp:                entry.Timestamp,
			Model:                    entry.Message.Model,
			InputTokens:              entry.Message.Usage.InputTokens,
			OutputTokens:             entry.Message.Usage.OutputTokens,
			CacheCreationInputTokens: entry.Message.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     entry.Message.Usage.CacheReadInputTokens,
			TotalTokens:              entry.Message.Usage.TotalTokens(),
			CostUSD:                  entry.CostUSD,
		}
		data.Entries = append(data.Entries, exportEntry)

		// Update summary.
		summary.TotalEntries++
		summary.TotalInputTokens += entry.Message.Usage.InputTokens
		summary.TotalOutputTokens += entry.Message.Usage.OutputTokens
		summary.TotalCacheCreationTokens += entry.Message.Usage.CacheCreationInputTokens
		summary.TotalCacheReadTokens += entry.Message.Usage.CacheReadInputTokens
		summary.TotalTokens += entry.Message.Usage.TotalTokens()

		if entry.CostUSD != nil {
			totalCost += *entry.CostUSD
		}

		// Track first and last entry timestamps.
		if summary.FirstEntry == "" || entry.Timestamp.Format(time.RFC3339) < summary.FirstEntry {
			summary.FirstEntry = entry.Timestamp.Format(time.RFC3339)
		}
		if entry.Timestamp.Format(time.RFC3339) > summary.LastEntry {
			summary.LastEntry = entry.Timestamp.Format(time.RFC3339)
		}
	}

	summary.TotalCostUSD = totalCost
	data.Summary = summary

	return data
}

// writeJSON writes export data as JSON.
func writeJSON(w *os.File, data ExportData) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// writeCSV writes export data as CSV.
func writeCSV(w *os.File, data ExportData) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header.
	header := []string{
		"timestamp",
		"session_id",
		"model",
		"input_tokens",
		"output_tokens",
		"cache_creation_tokens",
		"cache_read_tokens",
		"total_tokens",
		"cost_usd",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write entries.
	for _, entry := range data.Entries {
		costStr := ""
		if entry.CostUSD != nil {
			costStr = fmt.Sprintf("%.6f", *entry.CostUSD)
		}

		row := []string{
			entry.Timestamp.Format(time.RFC3339),
			data.SessionID,
			entry.Model,
			fmt.Sprintf("%d", entry.InputTokens),
			fmt.Sprintf("%d", entry.OutputTokens),
			fmt.Sprintf("%d", entry.CacheCreationInputTokens),
			fmt.Sprintf("%d", entry.CacheReadInputTokens),
			fmt.Sprintf("%d", entry.TotalTokens),
			costStr,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// showHelp displays session command help.
func (c *sessionCommand) showHelp() error {
	help := `Session Management Commands

Usage:
  token-monitor session <subcommand> [flags]

Subcommands:
  name <uuid> <name>    Assign a friendly name to a session
  list [flags]          List all sessions with metadata
  show <name|uuid>      Display detailed session information
  delete <name|uuid>    Remove session metadata (preserves data files)
  export <name|uuid>    Export session data to CSV or JSON format
  help                  Show this help message

List Flags:
  -sort        Sort by: name, date, uuid, tokens (default: name)
  -all         Show all sessions including unnamed
  -project     Filter by project path (substring match)
  -from        Filter sessions updated after date (YYYY-MM-DD)
  -to          Filter sessions updated before date (YYYY-MM-DD)
  -min-tokens  Filter sessions with at least N tokens
  -tokens      Show token counts in output

Delete Flags:
  -force   Skip confirmation prompt

Export Flags:
  -format  Output format: json, csv (default: json)
  -output  Output file path (default: stdout)

Examples:
  # Name a session
  token-monitor session name a1b2c3d4-e5f6-7890-abcd-ef1234567890 my-project

  # List named sessions
  token-monitor session list

  # List all sessions with token counts
  token-monitor session list -all -tokens

  # List sessions sorted by token usage
  token-monitor session list -sort tokens

  # Filter by project path
  token-monitor session list -project myapp

  # Filter by date range
  token-monitor session list -from 2025-11-01 -to 2025-11-30

  # Filter by minimum tokens
  token-monitor session list -min-tokens 10000

  # Combine filters
  token-monitor session list -project api -min-tokens 5000 -sort tokens

  # Show session details
  token-monitor session show my-project

  # Delete session metadata
  token-monitor session delete my-project

  # Export session to JSON (stdout)
  token-monitor session export my-project

  # Export session to JSON file
  token-monitor session export my-project -output session.json

  # Export session to CSV file
  token-monitor session export my-project -format csv -output session.csv
`
	fmt.Print(help)
	return nil
}
