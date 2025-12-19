package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primary   = lipgloss.Color("#7C3AED")
	secondary = lipgloss.Color("#A78BFA")
	success   = lipgloss.Color("#10B981")
	warning   = lipgloss.Color("#F59E0B")
	danger    = lipgloss.Color("#EF4444")
	muted     = lipgloss.Color("#6B7280")
	text      = lipgloss.Color("#F9FAFB")
	textDim   = lipgloss.Color("#9CA3AF")
	bg        = lipgloss.Color("#1F2937")
	bgDark    = lipgloss.Color("#111827")

	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Background(bgDark).
			Foreground(text)

	// Header styles
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primary).
			Padding(0, 1).
			MarginBottom(1)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text).
			Background(primary).
			Padding(0, 2)

	// List styles
	ListStyle = lipgloss.NewStyle().
			Padding(1, 2)

	SelectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(text).
				Background(primary).
				Padding(0, 1)

	NormalItemStyle = lipgloss.NewStyle().
			Foreground(text).
			Padding(0, 1)

	UnreadItemStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text).
			Padding(0, 1)

	// Email preview styles
	PreviewStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(muted).
			Padding(1, 2).
			MarginLeft(2)

	FromStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondary)

	SubjectStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text)

	DateStyle = lipgloss.NewStyle().
			Foreground(muted).
			Italic(true)

	SnippetStyle = lipgloss.NewStyle().
			Foreground(textDim)

	// Status bar styles
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(textDim).
			Background(bg).
			Padding(0, 1)

	StatusKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primary).
			Background(bg).
			Padding(0, 1)

	// Help styles
	HelpStyle = lipgloss.NewStyle().
			Foreground(muted).
			Padding(1, 2)

	HelpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondary)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(muted)

	// Loading/spinner styles
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(primary)

	// Error styles
	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(danger).
			Padding(1, 2)

	// Success styles
	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(success).
			Padding(1, 2)

	// Tab styles
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text).
			Background(primary).
			Padding(0, 2)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(muted).
				Background(bg).
				Padding(0, 2)

	// Badge styles
	UnreadBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(text).
			Background(primary).
			Padding(0, 1)

	LabelBadge = lipgloss.NewStyle().
			Foreground(text).
			Background(muted).
			Padding(0, 1)
)
