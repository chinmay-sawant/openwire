package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds Lip Gloss styles for the dark (default) UI.
type Theme struct {
	Name string

	App      lipgloss.Style
	Title    lipgloss.Style
	Border   lipgloss.Style
	Status   lipgloss.Style
	Muted    lipgloss.Style
	Accent   lipgloss.Style
	Selected lipgloss.Style
	Graph    lipgloss.Style
	Rx       lipgloss.Style
	Tx       lipgloss.Style
	Warn     lipgloss.Style
	Help     lipgloss.Style
	BarFill  lipgloss.Style
	BarEmpty lipgloss.Style
}

// DarkTheme is the default OpenWire palette.
func DarkTheme() Theme {
	bg := lipgloss.Color("#0d1117")
	fg := lipgloss.Color("#e6edf3")
	muted := lipgloss.Color("#8b949e")
	border := lipgloss.Color("#30363d")
	accent := lipgloss.Color("#58a6ff")
	green := lipgloss.Color("#3fb950")
	red := lipgloss.Color("#f85149")
	yellow := lipgloss.Color("#d29922")
	selBg := lipgloss.Color("#1f6feb")

	return Theme{
		Name: "dark",
		App: lipgloss.NewStyle().
			Foreground(fg).
			Background(bg),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			Background(bg),
		Border: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Foreground(fg).
			Background(bg),
		Status: lipgloss.NewStyle().
			Foreground(muted).
			Background(bg),
		Muted: lipgloss.NewStyle().
			Foreground(muted).
			Background(bg),
		Accent: lipgloss.NewStyle().
			Foreground(accent).
			Background(bg).
			Bold(true),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(selBg).
			Bold(true),
		Graph: lipgloss.NewStyle().
			Foreground(accent).
			Background(bg),
		Rx: lipgloss.NewStyle().
			Foreground(green).
			Background(bg),
		Tx: lipgloss.NewStyle().
			Foreground(red).
			Background(bg),
		Warn: lipgloss.NewStyle().
			Foreground(yellow).
			Background(bg),
		Help: lipgloss.NewStyle().
			Foreground(muted).
			Background(bg).
			Italic(true),
		BarFill: lipgloss.NewStyle().
			Foreground(accent).
			Background(bg),
		BarEmpty: lipgloss.NewStyle().
			Foreground(border).
			Background(bg),
	}
}
