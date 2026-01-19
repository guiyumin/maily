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
			HelpKeyStyle.Render("n") + HelpDescStyle.Render(" new email  ") +
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" reply  ") +
			HelpKeyStyle.Render("R") + HelpDescStyle.Render(" refresh  ") +
			HelpKeyStyle.Render("s") + HelpDescStyle.Render(" search  ") +
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit")
		row2 := HelpKeyStyle.Render("d") + HelpDescStyle.Render(" delete  ") +
			HelpKeyStyle.Render("l") + HelpDescStyle.Render(" load more  ") +
			HelpKeyStyle.Render("f") + HelpDescStyle.Render(" folders  ") +
			HelpKeyStyle.Render("/") + HelpDescStyle.Render(" commands")
		help = row1 + "\n" + row2
	} else {
		// Read view
		help = tabHint +
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" reply  ") +
			HelpKeyStyle.Render("u") + HelpDescStyle.Render(" unread  ") +
			HelpKeyStyle.Render("d") + HelpDescStyle.Render(" delete  ") +
			HelpKeyStyle.Render("a") + HelpDescStyle.Render(" attachments  ") +
			HelpKeyStyle.Render("s") + HelpDescStyle.Render(" summarize  ") +
			HelpKeyStyle.Render("e") + HelpDescStyle.Render(" create event  ") +
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

// RenderAISetupDialog renders a dialog asking user if they want to configure AI
func RenderAISetupDialog() string {
	dialogStyle := DialogStyle.BorderForeground(Primary)

	title := DialogTitleStyle.
		Foreground(Primary).
		Render("No AI Provider Found")

	message := lipgloss.NewStyle().
		Foreground(TextDim).
		Width(40).
		Align(lipgloss.Center).
		Render("Would you like to configure an AI provider?\n\nYou can add CLI tools (claude, codex, gemini) or API keys.")

	hint := DialogHintStyle.Render("Enter to configure, Esc to skip")

	return dialogStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			"",
			message,
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

func RenderSummaryDialog(width, height int, viewportContent string, provider string, scrollable bool) string {
	dialogWidth := min(width-20, 110)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	providerStyle := lipgloss.NewStyle().
		Foreground(Muted).
		Italic(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(Muted).
		MarginTop(1)

	hint := "Press Esc to close"
	if scrollable {
		hint = "j/k to scroll • Esc to close"
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Summary"),
		"",
		viewportContent,
		"",
		providerStyle.Render("via "+provider),
		"",
		hintStyle.Render(hint),
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
	Reminder  string // e.g., "15 minutes before" or empty
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

	reminderText := data.Reminder
	if reminderText == "" {
		reminderText = "No reminder set"
	}
	lines = append(lines, labelStyle.Render("Reminder:")+valueStyle.Render(reminderText))

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Extracted Event"),
		"",
		strings.Join(lines, "\n"),
		"",
		providerStyle.Render("via "+data.Provider),
		"",
		hintStyle.Render("Enter: add · e: edit · Esc: close"),
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

// ExtractEditData contains form data for editing extracted events
type ExtractEditData struct {
	TitleInput    string
	DateInput     string
	StartInput    string
	EndInput      string
	LocationInput string
	NotesInput    string
	ReminderIdx   int
	ReminderLabel string
	FocusIdx      int
	Provider      string
}

func RenderExtractEditDialog(width, height int, data ExtractEditData) string {
	dialogWidth := min(width-20, 60)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(Muted).
		Width(10)

	focusedLabelStyle := lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true).
		Width(10)

	inputStyle := lipgloss.NewStyle().
		Foreground(Text)

	providerStyle := lipgloss.NewStyle().
		Foreground(Muted).
		Italic(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(Muted).
		MarginTop(1)

	// Build form fields
	fields := []struct {
		label string
		value string
	}{
		{"Title:", data.TitleInput},
		{"Date:", data.DateInput},
		{"Start:", data.StartInput},
		{"End:", data.EndInput},
		{"Location:", data.LocationInput},
		{"Notes:", data.NotesInput},
	}

	var lines []string
	for i, f := range fields {
		ls := labelStyle
		if i == data.FocusIdx {
			ls = focusedLabelStyle
		}
		lines = append(lines, ls.Render(f.label)+inputStyle.Render(f.value))
	}

	// Add reminder field (uses ↑↓ to change)
	reminderLs := labelStyle
	reminderHint := ""
	if data.FocusIdx == 6 {
		reminderLs = focusedLabelStyle
		reminderHint = " (↑↓)"
	}
	lines = append(lines, reminderLs.Render("Reminder:")+inputStyle.Render(data.ReminderLabel+reminderHint))

	// Button styles
	buttonStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Muted)

	focusedButtonStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Foreground(Primary).
		Bold(true)

	// Build buttons
	saveStyle := buttonStyle
	cancelStyle := buttonStyle
	if data.FocusIdx == 7 {
		saveStyle = focusedButtonStyle
	}
	if data.FocusIdx == 8 {
		cancelStyle = focusedButtonStyle
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Center, saveStyle.Render("Save"), "  ", cancelStyle.Render("Cancel"))

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Edit Event"),
		"",
		strings.Join(lines, "\n"),
		"",
		buttons,
		"",
		providerStyle.Render("via "+data.Provider),
		"",
		hintStyle.Render("Tab: next · Enter: select · Esc: back"),
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

// WrapWithHangingIndent wraps text with proper hanging indent for list items.
// Lines starting with list markers (-, *, •, 1.) will have continuation lines
// indented to align with the text after the marker.
func WrapWithHangingIndent(text string, width int) string {
	if width <= 0 {
		return text
	}

	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		if line == "" {
			result = append(result, "")
			continue
		}

		// Detect list item and calculate indent
		indent := detectListIndent(line)
		if indent > 0 && indent < width/2 {
			// Wrap with hanging indent
			wrapped := wrapLineWithIndent(line, width, indent)
			result = append(result, wrapped...)
		} else {
			// Regular wrap
			wrapped := wrapLine(line, width)
			result = append(result, wrapped...)
		}
	}

	return strings.Join(result, "\n")
}

// detectListIndent returns the indent width for continuation lines.
// Handles list items (-, *, •, 1.) and general indented text.
func detectListIndent(line string) int {
	// Count leading spaces
	leadingSpaces := 0
	for _, c := range line {
		if c == ' ' {
			leadingSpaces++
		} else if c == '\t' {
			leadingSpaces += 4
		} else {
			break
		}
	}

	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return 0
	}

	// Check for bullet markers: -, *, •
	if len(trimmed) >= 2 && (trimmed[0] == '-' || trimmed[0] == '*' || strings.HasPrefix(trimmed, "•")) {
		markerLen := 1
		if strings.HasPrefix(trimmed, "• ") {
			markerLen = len("• ")
		} else if len(trimmed) > 1 && trimmed[1] == ' ' {
			markerLen = 2
		}
		return leadingSpaces + markerLen
	}

	// Check for numbered list: 1. 2. etc.
	for i, c := range trimmed {
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '.' && i > 0 && i < len(trimmed)-1 && trimmed[i+1] == ' ' {
			return leadingSpaces + i + 2 // number + "." + " "
		}
		break
	}

	// Preserve general indentation (e.g., "  summary text")
	if leadingSpaces > 0 {
		return leadingSpaces
	}

	return 0
}

// wrapLineWithIndent wraps a single line with hanging indent
func wrapLineWithIndent(line string, width, indent int) []string {
	if len(line) <= width {
		return []string{line}
	}

	var result []string
	indentStr := strings.Repeat(" ", indent)
	remaining := line
	isFirst := true

	for len(remaining) > 0 {
		maxWidth := width
		prefix := ""
		if !isFirst {
			prefix = indentStr
			maxWidth = width - indent
			if maxWidth <= 0 {
				maxWidth = width / 2
			}
		}

		if len(remaining) <= maxWidth {
			result = append(result, prefix+remaining)
			break
		}

		// Find break point (last space within width)
		breakPoint := maxWidth
		for i := maxWidth; i > 0; i-- {
			if remaining[i] == ' ' {
				breakPoint = i
				break
			}
		}

		// If no space found, force break at width
		if breakPoint == maxWidth && remaining[breakPoint] != ' ' {
			for i := maxWidth; i > maxWidth/2; i-- {
				if remaining[i] == ' ' {
					breakPoint = i
					break
				}
			}
		}

		result = append(result, prefix+remaining[:breakPoint])
		remaining = strings.TrimLeft(remaining[breakPoint:], " ")
		isFirst = false
	}

	return result
}

// wrapLine wraps a single line without special indent
func wrapLine(line string, width int) []string {
	if len(line) <= width {
		return []string{line}
	}

	var result []string
	remaining := line

	for len(remaining) > 0 {
		if len(remaining) <= width {
			result = append(result, remaining)
			break
		}

		// Find break point
		breakPoint := width
		for i := width; i > width/2; i-- {
			if remaining[i] == ' ' {
				breakPoint = i
				break
			}
		}

		result = append(result, remaining[:breakPoint])
		remaining = strings.TrimLeft(remaining[breakPoint:], " ")
	}

	return result
}
