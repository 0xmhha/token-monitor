// Package tui provides an interactive terminal UI for token-monitor
// using the Bubbletea framework (Elm Architecture: Model -> Update -> View).
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
	"github.com/0xmhha/token-monitor/pkg/config"
	"github.com/0xmhha/token-monitor/pkg/discovery"
	"github.com/0xmhha/token-monitor/pkg/logger"
	"github.com/0xmhha/token-monitor/pkg/monitor"
	"github.com/0xmhha/token-monitor/pkg/parser"
	"github.com/0xmhha/token-monitor/pkg/reader"
	"github.com/0xmhha/token-monitor/pkg/session"
	"github.com/0xmhha/token-monitor/pkg/watcher"
)

// Tab represents a navigation tab.
type Tab int

const (
	TabDashboard Tab = iota
	TabSessions
	TabStats
)

var tabNames = []string{"Dashboard", "Sessions", "Stats"}

// Messages

// monitorUpdateMsg carries a live monitor update.
type monitorUpdateMsg monitor.Update

// sessionsLoadedMsg carries discovered sessions.
type sessionsLoadedMsg []discovery.SessionFile

// statsLoadedMsg carries aggregated statistics.
type statsLoadedMsg struct {
	stats       aggregator.Statistics
	topSessions []aggregator.SessionStats
}

// sessionDetailLoadedMsg carries stats for a selected session (three-way split).
type sessionDetailLoadedMsg struct {
	sessionID    string
	filePath     string
	projectPath  string
	pastStats    aggregator.Statistics
	currentStats aggregator.Statistics
	totalStats   aggregator.Statistics
	burnRate     aggregator.BurnRate
	block        aggregator.BillingBlock
}

// tickMsg triggers periodic refresh.
type tickMsg time.Time

// errMsg carries an error.
type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// Model is the main application model.
type Model struct {
	// Navigation
	activeTab Tab
	showHelp  bool
	keys      KeyMap

	// Views
	dashboard dashboardView
	sessions  sessionsView
	statsView statsView

	// Infrastructure
	cfg        *config.Config
	log        logger.Logger
	sessionMgr session.Manager
	rdr        reader.Reader
	wtch       watcher.Watcher
	mon        monitor.LiveMonitor
	disc       discovery.Discoverer

	// State
	startTime time.Time // TUI start time, used as past/current boundary
	width     int
	height    int
	err       error
	ready     bool
}

// Options configures the TUI application.
type Options struct {
	SessionID string
	Refresh   time.Duration
	LogLevel  string
}

// New creates and runs the TUI application.
func New(opts Options) error {
	m, err := initModel(opts)
	if err != nil {
		return fmt.Errorf("failed to initialize TUI: %w", err)
	}

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

func initModel(opts Options) (Model, error) {
	cfg, err := config.Load()
	if err != nil {
		return Model{}, fmt.Errorf("failed to load config: %w", err)
	}

	logLevel := "error"
	if opts.LogLevel != "" {
		logLevel = opts.LogLevel
	}

	log := logger.New(logger.Config{
		Level:  logLevel,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	sessionMgr, err := session.New(session.Config{
		DBPath: cfg.Storage.DBPath,
	}, log)
	if err != nil {
		return Model{}, fmt.Errorf("failed to initialize session manager: %w", err)
	}

	positionStore, err := reader.NewBoltPositionStore(sessionMgr.DB())
	if err != nil {
		sessionMgr.Close()
		return Model{}, fmt.Errorf("failed to initialize position store: %w", err)
	}

	rdr, err := reader.New(reader.Config{
		PositionStore: positionStore,
		Parser:        parser.New(),
	}, log)
	if err != nil {
		sessionMgr.Close()
		return Model{}, fmt.Errorf("failed to initialize reader: %w", err)
	}

	wtch, err := watcher.New(watcher.Config{
		DebounceInterval: 100 * time.Millisecond,
	}, log)
	if err != nil {
		rdr.Close()
		sessionMgr.Close()
		return Model{}, fmt.Errorf("failed to initialize watcher: %w", err)
	}

	disc := discovery.New(cfg.ClaudeConfigDirs, log)

	refresh := opts.Refresh
	if refresh == 0 {
		refresh = time.Second
	}

	var sessionIDs []string
	if opts.SessionID != "" {
		sessionIDs = []string{opts.SessionID}
	}

	mon, err := monitor.New(monitor.Config{
		SessionIDs:      sessionIDs,
		RefreshInterval: refresh,
		ClearScreen:     false,
	}, wtch, rdr, disc, log)
	if err != nil {
		wtch.Close()
		rdr.Close()
		sessionMgr.Close()
		return Model{}, fmt.Errorf("failed to create monitor: %w", err)
	}

	return Model{
		activeTab:  TabDashboard,
		startTime:  time.Now(),
		keys:       DefaultKeyMap(),
		dashboard:  newDashboardView(),
		sessions:   newSessionsView(),
		statsView:  newStatsView(),
		cfg:        cfg,
		log:        log,
		sessionMgr: sessionMgr,
		rdr:        rdr,
		wtch:       wtch,
		mon:        mon,
		disc:       disc,
	}, nil
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.startMonitor(),
		m.loadSessions(),
		m.loadStats(),
	)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentHeight := msg.Height - 4 // Reserve for tabs + status bar
		m.dashboard.setSize(msg.Width, contentHeight)
		m.sessions.setSize(msg.Width, contentHeight)
		m.statsView.setSize(msg.Width, contentHeight)
		m.ready = true
		return m, nil

	case monitorUpdateMsg:
		upd := monitor.Update(msg)
		m.dashboard.update(upd)
		return m, m.waitForUpdate()

	case sessionsLoadedMsg:
		m.sessions.setSessions([]discovery.SessionFile(msg))
		return m, nil

	case sessionDetailLoadedMsg:
		m.dashboard.setDetail(&sessionDetail{
			sessionID:    msg.sessionID,
			filePath:     msg.filePath,
			projectPath:  msg.projectPath,
			pastStats:    msg.pastStats,
			currentStats: msg.currentStats,
			totalStats:   msg.totalStats,
			burnRate:     msg.burnRate,
			block:        msg.block,
		})
		m.activeTab = TabDashboard
		return m, nil

	case statsLoadedMsg:
		m.statsView.setStats(msg.stats)
		m.statsView.setTopSessions(msg.topSessions)
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.loadStats(), m.tick())

	case errMsg:
		m.err = msg.err
		return m, nil
	}

	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.showHelp {
		return renderHelp(m.width, m.height)
	}

	var b strings.Builder

	// Tab bar
	b.WriteString(m.renderTabs())
	b.WriteString("\n")

	// Content
	switch m.activeTab {
	case TabDashboard:
		b.WriteString(m.dashboard.view())
	case TabSessions:
		b.WriteString(m.sessions.view())
	case TabStats:
		b.WriteString(m.statsView.view())
	}

	// Status bar
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())

	return b.String()
}

// Key handling

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help toggle takes priority
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		m.cleanup()
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
		return m, nil

	case key.Matches(msg, m.keys.Tab):
		m.activeTab = Tab((int(m.activeTab) + 1) % len(tabNames))
		return m, nil

	case key.Matches(msg, m.keys.ShiftTab):
		m.activeTab = Tab((int(m.activeTab) - 1 + len(tabNames)) % len(tabNames))
		return m, nil

	case key.Matches(msg, m.keys.Number1):
		m.activeTab = TabDashboard
		return m, nil

	case key.Matches(msg, m.keys.Number2):
		m.activeTab = TabSessions
		return m, nil

	case key.Matches(msg, m.keys.Number3):
		m.activeTab = TabStats
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		// In session detail mode, only refresh that session
		if m.activeTab == TabDashboard && m.dashboard.hasDetail() {
			det := m.dashboard.detail
			return m, m.loadSessionDetail(det.sessionID, det.filePath, det.projectPath)
		}
		return m, tea.Batch(m.loadSessions(), m.loadStats())

	case key.Matches(msg, m.keys.Up):
		if m.activeTab == TabSessions {
			m.sessions.moveUp()
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.activeTab == TabSessions {
			m.sessions.moveDown()
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		if m.activeTab == TabSessions {
			if sess := m.sessions.selected(); sess != nil {
				return m, m.loadSessionDetail(sess.SessionID, sess.FilePath, sess.ProjectPath)
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		if m.activeTab == TabDashboard && m.dashboard.hasDetail() {
			m.dashboard.clearDetail()
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

// Rendering

func (m Model) renderTabs() string {
	var tabs []string
	for i, name := range tabNames {
		label := fmt.Sprintf(" %d:%s ", i+1, name)
		if Tab(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(label))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	gap := tabGapStyle.Render(strings.Repeat("─", max(0, m.width-lipgloss.Width(tabBar))))
	return tabBar + gap
}

func (m Model) renderStatusBar() string {
	left := statusKeyStyle.Render("q") + statusDescStyle.Render("quit") + " " +
		statusKeyStyle.Render("tab") + statusDescStyle.Render("switch") + " " +
		statusKeyStyle.Render("?") + statusDescStyle.Render("help") + " " +
		statusKeyStyle.Render("r") + statusDescStyle.Render("refresh")

	if m.activeTab == TabSessions {
		left += " " + statusKeyStyle.Render("enter") + statusDescStyle.Render("select")
	}
	if m.activeTab == TabDashboard && m.dashboard.hasDetail() {
		left += " " + statusKeyStyle.Render("esc") + statusDescStyle.Render("back to live")
	}

	right := ""
	if m.err != nil {
		right = dangerStyle.Render("Error: " + m.err.Error())
	}

	gap := max(0, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

// Commands

func (m Model) startMonitor() tea.Cmd {
	return func() tea.Msg {
		errCh := make(chan error, 1)
		go func() {
			if err := m.mon.Start(); err != nil {
				errCh <- err
			}
		}()

		// Wait briefly for startup errors
		select {
		case err := <-errCh:
			return errMsg{err}
		case <-time.After(200 * time.Millisecond):
		}

		// Return first update
		return m.receiveUpdate()
	}
}

func (m Model) waitForUpdate() tea.Cmd {
	return m.receiveUpdate
}

func (m Model) receiveUpdate() tea.Msg {
	liveMonitor, ok := m.mon.(interface{ Updates() <-chan monitor.Update })
	if !ok {
		return errMsg{fmt.Errorf("monitor does not support updates channel")}
	}

	select {
	case upd, ok := <-liveMonitor.Updates():
		if !ok {
			return nil
		}
		return monitorUpdateMsg(upd)
	case <-time.After(5 * time.Second):
		return tickMsg(time.Now())
	}
}

func (m Model) loadSessions() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.disc.Discover()
		if err != nil {
			return errMsg{err}
		}
		return sessionsLoadedMsg(sessions)
	}
}

func (m Model) loadStats() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.disc.Discover()
		if err != nil {
			return errMsg{err}
		}

		agg := aggregator.New(aggregator.Config{
			TrackPercentiles: true,
		})

		ctx := context.Background()
		for _, sess := range sessions {
			// Use ReadFrom(0) to read ALL data without affecting position tracking.
			// This prevents interference with the live monitor's incremental reads.
			entries, _, readErr := m.rdr.ReadFrom(ctx, sess.FilePath, 0)
			if readErr != nil {
				continue
			}
			for _, entry := range entries {
				agg.Add(entry)
			}
		}

		return statsLoadedMsg{
			stats:       agg.Stats(),
			topSessions: agg.TopSessions(10),
		}
	}
}

func (m Model) loadSessionDetail(sessionID, filePath, projectPath string) tea.Cmd {
	startTime := m.startTime
	return func() tea.Msg {
		ctx := context.Background()

		// Read ALL entries from offset 0 (not incremental)
		entries, _, err := m.rdr.ReadFrom(ctx, filePath, 0)
		if err != nil {
			return errMsg{err}
		}

		// Three aggregators: past, current, total
		pastAgg := aggregator.New(aggregator.Config{TrackPercentiles: true})
		curAgg := aggregator.New(aggregator.Config{TrackPercentiles: true})
		totalAgg := aggregator.New(aggregator.Config{TrackPercentiles: true})

		for _, entry := range entries {
			totalAgg.Add(entry)
			if entry.Timestamp.Before(startTime) {
				pastAgg.Add(entry)
			} else {
				curAgg.Add(entry)
			}
		}

		return sessionDetailLoadedMsg{
			sessionID:    sessionID,
			filePath:     filePath,
			projectPath:  projectPath,
			pastStats:    pastAgg.Stats(),
			currentStats: curAgg.Stats(),
			totalStats:   totalAgg.Stats(),
			burnRate:     totalAgg.BurnRate(sessionID, 5*time.Minute),
			block:        totalAgg.CurrentBillingBlock(sessionID),
		}
	}
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) cleanup() {
	if m.mon != nil {
		m.mon.Stop()
	}
	if m.wtch != nil {
		m.wtch.Close()
	}
	if m.rdr != nil {
		m.rdr.Close()
	}
	if m.sessionMgr != nil {
		m.sessionMgr.Close()
	}
}
