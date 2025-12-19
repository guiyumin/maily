package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Data structs for render functions

type HeaderData struct {
	Width          int
	Accounts       []string
	ActiveIdx      int
	IsSearchResult bool
	SearchQuery    string
}

type StatusBarData struct {
	Width          int
	StatusMsg      string
	SearchMode     bool
	IsSearchResult bool
	IsListView     bool
	AccountCount   int
	SelectionCount int
}

type EmailViewData struct {
	From    string
	To      string
	Subject string
	Date    time.Time
}

// Render functions

func RenderHeader(data HeaderData) string {
	title := TitleStyle.Render(" MAILY ")

	// Show search indicator if in search mode
	if data.IsSearchResult {
		searchBadge := lipgloss.NewStyle().
			Foreground(Text).
			Background(Warning).
			Padding(0, 1).
			Render(fmt.Sprintf(" Search: %s ", data.SearchQuery))
		return HeaderStyle.Width(data.Width).Render(title + " " + searchBadge)
	}

	var tabs []string
	activeTabStyle := lipgloss.NewStyle().
		Foreground(Text).
		Background(Primary).
		Padding(0, 1)
	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(TextDim).
		Padding(0, 1)

	for i, email := range data.Accounts {
		if i == data.ActiveIdx {
			tabs = append(tabs, activeTabStyle.Render(email))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(email))
		}
	}

	tabsStr := strings.Join(tabs, " ")
	return HeaderStyle.Width(data.Width).Render(title + " " + tabsStr)
}

func RenderStatusBar(data StatusBarData) string {
	var help string
	tabHint := ""
	if data.AccountCount > 1 && !data.IsSearchResult {
		tabHint = HelpKeyStyle.Render("tab") + HelpDescStyle.Render(" switch  ")
	}

	if data.SearchMode {
		help = HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" search  ") +
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" cancel")
	} else if data.IsSearchResult {
		help = HelpKeyStyle.Render("space") + HelpDescStyle.Render(" select  ") +
			HelpKeyStyle.Render("a") + HelpDescStyle.Render(" all  ") +
			HelpKeyStyle.Render("m") + HelpDescStyle.Render(" mark read  ") +
			HelpKeyStyle.Render("d") + HelpDescStyle.Render(" delete  ") +
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back  ") +
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit")
	} else if data.IsListView {
		help = tabHint +
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" navigate  ") +
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" open  ") +
			HelpKeyStyle.Render("/") + HelpDescStyle.Render(" search  ") +
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" refresh  ") +
			HelpKeyStyle.Render("d") + HelpDescStyle.Render(" delete  ") +
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit")
	} else {
		help = tabHint +
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back  ") +
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll  ") +
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit")
	}

	status := StatusKeyStyle.Render(data.StatusMsg)

	// Show selection count in search mode
	selectionInfo := ""
	if data.IsSearchResult && data.SelectionCount > 0 {
		selectionInfo = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981")).
			Render(fmt.Sprintf(" %d selected ", data.SelectionCount))
	}

	gap := max(0, data.Width-lipgloss.Width(help)-lipgloss.Width(status)-lipgloss.Width(selectionInfo)-12)

	return StatusBarStyle.Width(data.Width).PaddingLeft(4).PaddingRight(4).Render(
		help + strings.Repeat(" ", gap) + selectionInfo + status,
	)
}

func RenderListView(width, height int, listContent string) string {
	return lipgloss.NewStyle().
		Width(width).
		Height(height - 6).
		Render(listContent)
}

func RenderReadView(email EmailViewData, width int, viewportContent string) string {
	headerContent := lipgloss.JoinVertical(
		lipgloss.Left,
		FromStyle.Render("From: ")+email.From,
		"To: "+email.To,
		SubjectStyle.Render("Subject: ")+email.Subject,
		DateStyle.Render(email.Date.Format("Mon, 02 Jan 2006 15:04:05")),
		strings.Repeat("─", width-12),
	)

	header := lipgloss.NewStyle().
		PaddingLeft(4).
		PaddingRight(4).
		Render(headerContent)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		viewportContent,
	)
}

func RenderConfirmDialog(count int) string {
	dialogStyle := DialogStyle.BorderForeground(Danger)

	titleText := "Delete Email?"
	if count > 1 {
		titleText = fmt.Sprintf("Delete %d Emails?", count)
	}

	title := DialogTitleStyle.
		Foreground(Danger).
		Render(titleText)

	hint := DialogHintStyle.Render("press y to confirm, n to cancel")

	return dialogStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			"",
			hint,
		),
	)
}

func RenderSearchInput(inputView string) string {
	dialogStyle := DialogStyle.BorderForeground(Primary)

	title := DialogTitleStyle.
		Foreground(Primary).
		Render("Search")

	hint := DialogHintStyle.Render("Enter to search, Esc to cancel")

	return dialogStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			"",
			inputView,
			"",
			hint,
		),
	)
}

func RenderLoading(width, height int, spinnerView, statusMsg string) string {
	return lipgloss.Place(
		width,
		height-4,
		lipgloss.Center,
		lipgloss.Center,
		fmt.Sprintf("%s %s", spinnerView, statusMsg),
	)
}

func RenderError(width, height int, err error) string {
	return lipgloss.Place(
		width,
		height-4,
		lipgloss.Center,
		lipgloss.Center,
		ErrorStyle.Render(fmt.Sprintf("Error: %v", err)),
	)
}

func RenderCentered(width, height int, content string) string {
	return lipgloss.Place(
		width,
		height-4,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}
