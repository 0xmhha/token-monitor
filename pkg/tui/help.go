package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderHelp builds the help overlay content.
func renderHelp(width, height int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary).
		Render("Keyboard Shortcuts")

	sections := []struct {
		header string
		keys   [][]string
	}{
		{
			header: "Navigation",
			keys: [][]string{
				{"tab / shift+tab", "Switch tabs"},
				{"1 / 2 / 3", "Jump to tab"},
				{"up/k  down/j", "Navigate list"},
				{"enter", "Select item"},
				{"esc", "Back / close"},
			},
		},
		{
			header: "Actions",
			keys: [][]string{
				{"r", "Refresh data"},
				{"?", "Toggle help"},
				{"q / ctrl+c", "Quit"},
			},
		},
	}

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "")

	for _, section := range sections {
		header := lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSecondary).
			Render(section.header)
		lines = append(lines, header)

		for _, kv := range section.keys {
			k := helpKeyStyle.Render(kv[0])
			d := helpDescStyle.Render(kv[1])
			lines = append(lines, k+d)
		}
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")

	overlay := helpOverlayStyle.
		Width(min(50, width-4)).
		Render(content)

	// Center the overlay
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		overlay,
	)
}
