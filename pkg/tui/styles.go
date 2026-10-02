package tui

import "github.com/charmbracelet/lipgloss"

// Styles holds all visual styling for the TUI.
type Styles struct {
	App            lipgloss.Style
	Title          lipgloss.Style
	TitleBar       lipgloss.Style
	Footer         lipgloss.Style
	FooterText     lipgloss.Style
	HelpKey        lipgloss.Style
	HelpDesc       lipgloss.Style
	Dialog         lipgloss.Style
	DialogTitle    lipgloss.Style
	DialogText     lipgloss.Style
	ButtonActive   lipgloss.Style
	ButtonInactive lipgloss.Style
	DetailView     lipgloss.Style
	DetailTitle    lipgloss.Style
	DetailLabel    lipgloss.Style
	DetailValue    lipgloss.Style
	StatusOK       lipgloss.Style
	StatusDanger   lipgloss.Style
	Loading        lipgloss.Style
}

// DefaultStyles returns the default color theme.
func DefaultStyles() Styles {
	return Styles{
		App: lipgloss.NewStyle().
			Padding(0, 1),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("27")).
			Padding(0, 2),

		TitleBar: lipgloss.NewStyle().
			Background(lipgloss.Color("27")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Padding(0, 1),

		FooterText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")),

		HelpKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")),

		Dialog: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196")).
			Padding(1, 2),

		DialogTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true),

		DialogText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),

		ButtonActive: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("196")).
			Padding(0, 2).
			Bold(true),

		ButtonInactive: lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Padding(0, 2),

		DetailView: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")).
			Padding(1, 2),

		DetailTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true).
			Underline(true),

		DetailLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Width(14),

		DetailValue: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),

		StatusOK: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")),

		StatusDanger: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true),

		Loading: lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true),
	}
}
