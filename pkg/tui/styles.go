package tui

import "github.com/charmbracelet/lipgloss"

// Color palette.
var (
	colorPrimary   = lipgloss.Color("#7C3AED") // purple
	colorSecondary = lipgloss.Color("#06B6D4") // cyan
	colorSuccess   = lipgloss.Color("#22C55E") // green
	colorWarning   = lipgloss.Color("#F59E0B") // amber
	colorDanger    = lipgloss.Color("#EF4444") // red
	colorMuted     = lipgloss.Color("#6B7280") // gray
	colorText      = lipgloss.Color("#F9FAFB") // white
	colorSubtext   = lipgloss.Color("#9CA3AF") // light gray
	colorBgAlt     = lipgloss.Color("#1F2937") // dark alt
	colorBorder    = lipgloss.Color("#374151") // border gray
)

// Tab styles.
var (
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText).
			Background(colorPrimary).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(colorSubtext).
				Background(colorBgAlt).
				Padding(0, 2)

	tabGapStyle = lipgloss.NewStyle().
			Foreground(colorBorder)
)

// Panel styles.
var (
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1)

	highlightPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(1, 2)
)

// Text styles.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorSubtext)

	valueStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	warningStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	dangerStyle = lipgloss.NewStyle().
			Foreground(colorDanger)

	accentStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)
)

// Status bar styles.
var (
	statusKeyStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorBgAlt).
			Bold(true).
			Padding(0, 1)

	statusDescStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Background(colorBgAlt).
			Padding(0, 1)
)

// Table styles.
var (
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)

	tableSelectedStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorPrimary).
				Bold(true)
)

// Help styles.
var (
	helpOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorPrimary).
				Padding(1, 3).
				Align(lipgloss.Center)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true).
			Width(14)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorSubtext)
)
