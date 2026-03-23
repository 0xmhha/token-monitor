package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/0xmhha/token-monitor/pkg/discovery"
)

// sessionsView renders the interactive session list.
type sessionsView struct {
	sessions []discovery.SessionFile
	cursor   int
	offset   int // scroll offset
	width    int
	height   int
}

func newSessionsView() sessionsView {
	return sessionsView{}
}

func (s *sessionsView) setSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *sessionsView) setSessions(sessions []discovery.SessionFile) {
	s.sessions = sessions
	if s.cursor >= len(sessions) {
		s.cursor = max(0, len(sessions)-1)
	}
}

func (s *sessionsView) moveUp() {
	if s.cursor > 0 {
		s.cursor--
		if s.cursor < s.offset {
			s.offset = s.cursor
		}
	}
}

func (s *sessionsView) moveDown() {
	if s.cursor < len(s.sessions)-1 {
		s.cursor++
		visibleRows := s.visibleRows()
		if s.cursor >= s.offset+visibleRows {
			s.offset = s.cursor - visibleRows + 1
		}
	}
}

func (s *sessionsView) selected() *discovery.SessionFile {
	if s.cursor >= 0 && s.cursor < len(s.sessions) {
		return &s.sessions[s.cursor]
	}
	return nil
}

func (s *sessionsView) visibleRows() int {
	available := s.height - 6
	if available < 1 {
		return 1
	}
	return available
}

func (s *sessionsView) view() string {
	if len(s.sessions) == 0 {
		return lipgloss.Place(
			s.width, s.height,
			lipgloss.Center, lipgloss.Center,
			mutedStyle.Render("No sessions found"),
		)
	}

	var lines []string

	title := titleStyle.Render(fmt.Sprintf("Sessions (%d)", len(s.sessions)))
	lines = append(lines, title, "")

	// Dynamic column widths based on terminal width
	colNum := 5
	colSession := 38 // full UUID
	colProject := max(20, s.width-colNum-colSession-6)

	// Header row using lipgloss cells for correct alignment
	headerRow := "  " +
		cellLeft("#", colNum, tableHeaderStyle) +
		cellLeft("Session ID", colSession, tableHeaderStyle) +
		cellLeft("Project", colProject, tableHeaderStyle)
	lines = append(lines, headerRow)

	// Rows
	visible := s.visibleRows()
	end := min(s.offset+visible, len(s.sessions))

	for i := s.offset; i < end; i++ {
		sess := s.sessions[i]

		num := fmt.Sprintf("%d", i+1)
		project := shortenProjectPath(sess.ProjectPath)

		row := "  " +
			cellLeft(num, colNum, mutedStyle) +
			cellLeft(sess.SessionID, colSession, lipgloss.NewStyle().Foreground(colorText)) +
			cellLeft(project, colProject, subtitleStyle)

		if i == s.cursor {
			row = tableSelectedStyle.Width(s.width - 2).Render(
				"  " +
					cellLeft(num, colNum, tableSelectedStyle) +
					cellLeft(sess.SessionID, colSession, tableSelectedStyle) +
					cellLeft(project, colProject, tableSelectedStyle),
			)
		}

		lines = append(lines, row)
	}

	// Scroll indicator
	if len(s.sessions) > visible {
		scrollInfo := mutedStyle.Render(fmt.Sprintf("  Showing %d-%d of %d (scroll with j/k)",
			s.offset+1, end, len(s.sessions)))
		lines = append(lines, "", scrollInfo)
	}

	return strings.Join(lines, "\n")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
