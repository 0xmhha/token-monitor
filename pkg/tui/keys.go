package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the application.
type KeyMap struct {
	Quit      key.Binding
	Help      key.Binding
	Tab       key.Binding
	ShiftTab  key.Binding
	Refresh   key.Binding
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Escape    key.Binding
	PageUp    key.Binding
	PageDown  key.Binding
	Number1   key.Binding
	Number2   key.Binding
	Number3   key.Binding
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev tab"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdn", "page down"),
		),
		Number1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "dashboard"),
		),
		Number2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "sessions"),
		),
		Number3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "stats"),
		),
	}
}
