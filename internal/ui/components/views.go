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
	CurrentLabel   string
}

type StatusBarData struct {
	Width          int
	StatusMsg      string
	SearchMode     bool
	IsSearchResult bool
	IsListView     bool
	IsComposeView    bool
	AccountCount   int
	SelectionCount int
}

type AttachmentInfo struct {
	Filename    string
	ContentType string
	Size        int64
}

type EmailViewData struct {
	From        string
	To          string
	Subject     string
	Date        time.Time
	Attachments []AttachmentInfo
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

	// Show current label badge if not in inbox
	labelBadge := ""
	if data.CurrentLabel != "" && data.CurrentLabel != "INBOX" {
		labelName := GetLabelDisplayName(data.CurrentLabel)
		labelBadge = " " + lipgloss.NewStyle().
			Foreground(Text).
			Background(Secondary).
			Padding(0, 1).
			Render(labelName)
	}

	return HeaderStyle.Width(data.Width).Render(title + " " + tabsStr + labelBadge)
}

func RenderStatusBar(data StatusBarData) string {
	var help string
	tabHint := ""
	if data.AccountCount > 1 && !data.IsSearchResult && !data.IsComposeView {
		tabHint = HelpKeyStyle.Render("tab") + HelpDescStyle.Render(" switch  ")
	}

	if data.SearchMode {
		help = HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" search  ") +
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" cancel")
	} else if data.IsComposeView {
		help = HelpKeyStyle.Render("Tab") + HelpDescStyle.Render(" next field")
	} else if data.IsSearchResult {
		help = HelpKeyStyle.Render("space") + HelpDescStyle.Render(" select  ") +
			HelpKeyStyle.Render("a") + HelpDescStyle.Render(" all  ") +
			HelpKeyStyle.Render("m") + HelpDescStyle.Render(" mark read  ") +
			HelpKeyStyle.Render("d") + HelpDescStyle.Render(" delete  ") +
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back  ") +
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit")
	} else if data.IsListView {
		row1 := tabHint +
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" open  ") +
			HelpKeyStyle.Render("c") + HelpDescStyle.Render(" compose  ") +
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" reply  ") +
			HelpKeyStyle.Render("R") + HelpDescStyle.Render(" refresh  ") +
			HelpKeyStyle.Render("s") + HelpDescStyle.Render(" search  ") +
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit")
		row2 := HelpKeyStyle.Render("d") + HelpDescStyle.Render(" delete  ") +
			HelpKeyStyle.Render("l") + HelpDescStyle.Render(" more  ") +
			HelpKeyStyle.Render("f") + HelpDescStyle.Render(" folders  ") +
			HelpKeyStyle.Render("/") + HelpDescStyle.Render(" commands")
		help = row1 + "\n" + row2
	} else {
		// Read view
		help = tabHint +
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" reply  ") +
			HelpKeyStyle.Render("a") + HelpDescStyle.Render(" download attachments ") +
			HelpKeyStyle.Render("s") + HelpDescStyle.Render(" summarize  ") +
			HelpKeyStyle.Render("/") + HelpDescStyle.Render(" commands  ") +
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back  ") +
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

	return StatusBarStyle.Width(data.Width).PaddingLeft(4).PaddingRight(4).MarginTop(1).Render(
		help + strings.Repeat(" ", gap) + selectionInfo + status,
	)
}

func RenderListView(width, height int, listContent string) string {
	// Don't set fixed Height - let content determine height
	// The mailList already limits visible rows based on its height
	return lipgloss.NewStyle().
		Width(width).
		Render(listContent)
}

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func RenderReadView(email EmailViewData, width int, viewportContent string) string {
	headerLines := []string{
		FromStyle.Render("From: ") + email.From,
		"To: " + email.To,
		SubjectStyle.Render("Subject: ") + email.Subject,
		DateStyle.Render(email.Date.Format("Mon, 02 Jan 2006 15:04:05")),
	}

	// Add attachments line if there are any
	if len(email.Attachments) > 0 {
		attachStyle := lipgloss.NewStyle().Foreground(Secondary).Bold(true)
		fileStyle := lipgloss.NewStyle().Foreground(Text)
		sizeStyle := lipgloss.NewStyle().Foreground(Muted)

		var attachParts []string
		for _, att := range email.Attachments {
			attachParts = append(attachParts, fmt.Sprintf("%s %s",
				fileStyle.Render(att.Filename),
				sizeStyle.Render("("+formatFileSize(att.Size)+")")))
		}

		attachLine := attachStyle.Render("📎 Attachments: ") + strings.Join(attachParts, ", ")
		headerLines = append(headerLines, attachLine)
	}

	headerLines = append(headerLines, strings.Repeat("─", width-12))

	headerContent := lipgloss.JoinVertical(lipgloss.Left, headerLines...)

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

// DeleteOption represents the selected delete action
type DeleteOption int

const (
	DeleteOptionTrash DeleteOption = iota
	DeleteOptionPermanent
	DeleteOptionCancel
)

func RenderConfirmDialog(count int, selected DeleteOption) string {
	dialogStyle := DialogStyle.BorderForeground(Warning)

	titleText := "Delete Email?"
	if count > 1 {
		titleText = fmt.Sprintf("Delete %d Emails?", count)
	}

	title := DialogTitleStyle.
		Foreground(Warning).
		Render(titleText)

	// Button styles
	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Bg).
		Background(Primary).
		Padding(0, 2)

	unselectedStyle := lipgloss.NewStyle().
		Foreground(Text).
		Padding(0, 2)

	dangerSelectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Bg).
		Background(Danger).
		Padding(0, 2)

	// Render buttons
	var trashBtn, permBtn, cancelBtn string
	if selected == DeleteOptionTrash {
		trashBtn = selectedStyle.Render("Move to Trash")
	} else {
		trashBtn = unselectedStyle.Render("Move to Trash")
	}
	if selected == DeleteOptionPermanent {
		permBtn = dangerSelectedStyle.Render("Permanent Delete")
	} else {
		permBtn = unselectedStyle.Render("Permanent Delete")
	}
	if selected == DeleteOptionCancel {
		cancelBtn = selectedStyle.Render("Cancel")
	} else {
		cancelBtn = unselectedStyle.Render("Cancel")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, trashBtn, "  ", permBtn, "  ", cancelBtn)

	hint := DialogHintStyle.Render("← → to select, enter to confirm, esc to cancel")

	return dialogStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			"",
			buttons,
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

func RenderError(width, height int, err error, accountEmail string, canSwitch bool) string {
	errorText := fmt.Sprintf("Error: %v", err)
	if accountEmail != "" {
		errorText = fmt.Sprintf("Error [%s]: %v", accountEmail, err)
	}

	// Check if this is a login/authentication error
	errStr := err.Error()
	isAuthError := strings.Contains(errStr, "login failed") ||
		strings.Contains(errStr, "AUTHENTICATIONFAILED") ||
		strings.Contains(errStr, "Invalid credentials")

	fixHint := ""
	if isAuthError {
		fixHintStyle := lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)
		fixHint = "\n\n" + fixHintStyle.Render("To fix: Generate a new App Password for your email provider") +
			"\n" + fixHintStyle.Render("Then run: maily login")
	}

	hint := ""
	if canSwitch {
		hint = "\n\n" + HelpKeyStyle.Render("tab") + HelpDescStyle.Render(" switch account  ") +
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit")
	} else {
		hint = "\n\n" + HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit")
	}

	return lipgloss.Place(
		width,
		height-4,
		lipgloss.Center,
		lipgloss.Center,
		ErrorStyle.Render(errorText)+fixHint+hint,
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

func RenderSummaryDialog(width, height int, summary string, provider string) string {
	dialogWidth := min(width-20, 70)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	contentStyle := lipgloss.NewStyle().
		Width(dialogWidth - 8).
		Foreground(Text)

	providerStyle := lipgloss.NewStyle().
		Foreground(Muted).
		Italic(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(Muted).
		MarginTop(1)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Summary"),
		"",
		contentStyle.Render(summary),
		"",
		providerStyle.Render("via "+provider),
		"",
		hintStyle.Render("Press Esc to close"),
	)

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(1, 3).
		Width(dialogWidth)

	return lipgloss.Place(
		width,
		height-4,
		lipgloss.Center,
		lipgloss.Center,
		dialogStyle.Render(content),
	)
}

func RenderExtractInputDialog(width, height int, inputView string) string {
	dialogWidth := min(width-20, 60)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(Muted).
		MarginBottom(1)

	hintStyle := lipgloss.NewStyle().
		Foreground(Muted).
		MarginTop(1)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("No Event Found"),
		subtitleStyle.Render("Type event details to add to calendar:"),
		"",
		"  "+inputView,
		"",
		hintStyle.Render("enter to parse • esc to cancel"),
	)

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(1, 3).
		Width(dialogWidth)

	return lipgloss.Place(
		width,
		height-4,
		lipgloss.Center,
		lipgloss.Center,
		dialogStyle.Render(content),
	)
}

// ExtractData holds the extracted event data for rendering
type ExtractData struct {
	Title     string
	StartTime time.Time
	EndTime   time.Time
	Location  string
	Provider  string
}

func RenderExtractDialog(width, height int, data ExtractData) string {
	dialogWidth := min(width-20, 60)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(Muted).
		Width(10)

	valueStyle := lipgloss.NewStyle().
		Foreground(Text)

	providerStyle := lipgloss.NewStyle().
		Foreground(Muted).
		Italic(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(Muted).
		MarginTop(1)

	// Format event details
	dateStr := data.StartTime.Format("Monday, Jan 2, 2006")
	timeStr := fmt.Sprintf("%s - %s", data.StartTime.Format("3:04 PM"), data.EndTime.Format("3:04 PM"))

	lines := []string{
		labelStyle.Render("Title:") + valueStyle.Render(data.Title),
		labelStyle.Render("Date:") + valueStyle.Render(dateStr),
		labelStyle.Render("Time:") + valueStyle.Render(timeStr),
	}

	if data.Location != "" {
		lines = append(lines, labelStyle.Render("Location:")+valueStyle.Render(data.Location))
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Extracted Event"),
		"",
		strings.Join(lines, "\n"),
		"",
		providerStyle.Render("via "+data.Provider),
		"",
		hintStyle.Render("Press Esc to close"),
	)

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(1, 3).
		Width(dialogWidth)

	return lipgloss.Place(
		width,
		height-4,
		lipgloss.Center,
		lipgloss.Center,
		dialogStyle.Render(content),
	)
}

func RenderAttachmentPicker(width, height int, attachments []AttachmentInfo, selectedIdx int) string {
	dialogWidth := min(width-20, 60)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Bg).
		Background(Primary).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(Text).
		Padding(0, 1)

	sizeStyle := lipgloss.NewStyle().
		Foreground(Muted)

	hintStyle := lipgloss.NewStyle().
		Foreground(Muted).
		MarginTop(1)

	var items []string

	// First item: Download All
	totalSize := int64(0)
	for _, att := range attachments {
		totalSize += att.Size
	}
	downloadAllText := fmt.Sprintf("Download All (%d files, %s)", len(attachments), formatFileSize(totalSize))
	if selectedIdx == 0 {
		items = append(items, selectedStyle.Render("→ "+downloadAllText))
	} else {
		items = append(items, normalStyle.Render("  "+downloadAllText))
	}

	// Individual attachments (index shifted by 1)
	for i, att := range attachments {
		line := fmt.Sprintf("%s %s", att.Filename, sizeStyle.Render("("+formatFileSize(att.Size)+")"))
		if i+1 == selectedIdx {
			items = append(items, selectedStyle.Render("→ "+line))
		} else {
			items = append(items, normalStyle.Render("  "+line))
		}
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("📎 Download Attachments"),
		"",
		strings.Join(items, "\n"),
		"",
		hintStyle.Render("tab select  enter download  esc cancel"),
	)

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(1, 2).
		Width(dialogWidth)

	return lipgloss.Place(
		width,
		height-4,
		lipgloss.Center,
		lipgloss.Center,
		dialogStyle.Render(content),
	)
}
