package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"maily/internal/i18n"
	"maily/internal/ui/components"
)

func (m *CalendarApp) renderForm(title string) string {
	var b strings.Builder

	// Title header
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(components.Primary).
		MarginBottom(1)
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Build form content for the box
	var content strings.Builder

	labelStyle := lipgloss.NewStyle().Width(11).Foreground(components.Muted)
	focusedLabelStyle := lipgloss.NewStyle().Width(11).Foreground(components.Primary).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(components.Muted)

	// Title field
	if m.formFocusIdx == 0 {
		content.WriteString(focusedLabelStyle.Render(i18n.T("calendar.field.title")))
	} else {
		content.WriteString(labelStyle.Render(i18n.T("calendar.field.title")))
	}
	content.WriteString(m.form.title.View())
	content.WriteString("\n")

	// Date field
	if m.formFocusIdx == 1 {
		content.WriteString(focusedLabelStyle.Render(i18n.T("calendar.field.date")))
	} else {
		content.WriteString(labelStyle.Render(i18n.T("calendar.field.date")))
	}
	content.WriteString(m.form.date.View())
	if m.formFocusIdx == 1 {
		content.WriteString(hintStyle.Render("  ↑↓←→"))
	}
	content.WriteString("\n")

	// Start time
	if m.formFocusIdx == 2 {
		content.WriteString(focusedLabelStyle.Render(i18n.T("calendar.field.start")))
	} else {
		content.WriteString(labelStyle.Render(i18n.T("calendar.field.start")))
	}
	content.WriteString(m.form.start.View())
	if m.formFocusIdx == 2 {
		content.WriteString(hintStyle.Render("  ↑↓←→"))
	}
	content.WriteString("\n")

	// End time
	if m.formFocusIdx == 3 {
		content.WriteString(focusedLabelStyle.Render(i18n.T("calendar.field.end")))
	} else {
		content.WriteString(labelStyle.Render(i18n.T("calendar.field.end")))
	}
	content.WriteString(m.form.end.View())
	if m.formFocusIdx == 3 {
		content.WriteString(hintStyle.Render("  ↑↓←→"))
	}
	content.WriteString("\n")

	// Location
	if m.formFocusIdx == 4 {
		content.WriteString(focusedLabelStyle.Render(i18n.T("calendar.field.location")))
	} else {
		content.WriteString(labelStyle.Render(i18n.T("calendar.field.location")))
	}
	content.WriteString(m.form.location.View())
	content.WriteString("\n")

	// Notes
	if m.formFocusIdx == 5 {
		content.WriteString(focusedLabelStyle.Render(i18n.T("calendar.field.notes")))
	} else {
		content.WriteString(labelStyle.Render(i18n.T("calendar.field.notes")))
	}
	content.WriteString(m.form.notes.View())
	content.WriteString("\n")

	// Calendar selector
	if m.formFocusIdx == 6 {
		content.WriteString(focusedLabelStyle.Render(i18n.T("calendar.field.calendar")))
	} else {
		content.WriteString(labelStyle.Render(i18n.T("calendar.field.calendar")))
	}

	calName := i18n.T("calendar.default")
	if len(m.calendars) > 0 && m.form.calendar < len(m.calendars) {
		calName = m.calendars[m.form.calendar].Title
	}
	calStyle := lipgloss.NewStyle()
	if m.formFocusIdx == 6 {
		calStyle = calStyle.Background(components.Primary).Foreground(components.Text)
	}
	content.WriteString(calStyle.Render(fmt.Sprintf("◀ %s ▶", calName)))

	// Error (inside box)
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(components.Danger)
		content.WriteString("\n\n")
		content.WriteString(errStyle.Render(fmt.Sprintf("%s: %v", i18n.T("common.error"), m.err)))
	}

	// Create bordered box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(components.Primary).
		Padding(1, 2).
		Width(50)

	b.WriteString(boxStyle.Render(content.String()))
	b.WriteString("\n\n")

	// Save and Cancel buttons - highlight selected one with borders
	selectedBtn := lipgloss.NewStyle().
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		Padding(0, 2)
	unselectedBtn := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(components.Muted).
		Padding(0, 2).
		Foreground(components.Muted)

	var saveBtn, cancelBtn string
	if m.formFocusIdx == 7 {
		saveBtn = selectedBtn.BorderForeground(components.Primary).Background(components.Primary).Foreground(lipgloss.Color("#FFFFFF")).Render(i18n.T("common.save"))
	} else {
		saveBtn = unselectedBtn.Render(i18n.T("common.save"))
	}
	if m.formFocusIdx == 8 {
		cancelBtn = selectedBtn.BorderForeground(components.Muted).Background(components.Muted).Foreground(lipgloss.Color("#FFFFFF")).Render(i18n.T("common.cancel"))
	} else {
		cancelBtn = unselectedBtn.Render(i18n.T("common.cancel"))
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, saveBtn, "  ", cancelBtn))
	b.WriteString("\n\n")

	// Help hint
	helpStyle := lipgloss.NewStyle().Foreground(components.Muted)
	b.WriteString(helpStyle.Render(fmt.Sprintf("Tab: %s • Enter: %s • Ctrl+S: %s • Esc: %s", i18n.T("calendar.cycle"), i18n.T("help.select"), i18n.T("common.save"), i18n.T("help.cancel"))))

	// Wrap with padding
	formStyle := lipgloss.NewStyle().Padding(1, 2)
	return formStyle.Render(b.String())
}

func (m *CalendarApp) renderDeleteConfirm() string {
	var b strings.Builder

	dayEvents := m.eventsForDate(m.selectedDate)
	var eventTitle string
	if m.selectedIdx < len(dayEvents) {
		eventTitle = dayEvents[m.selectedIdx].Title
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(components.Danger)

	b.WriteString(titleStyle.Render(i18n.T("calendar.delete_event")))
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "%s\n\n", i18n.T("calendar.delete_confirm", map[string]any{"Title": eventTitle}))

	// Button styles
	selectedBtn := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(components.Danger).
		Padding(0, 2)
	unselectedBtn := lipgloss.NewStyle().
		Foreground(components.Muted).
		Padding(0, 2)

	// Render buttons
	var deleteBtn, cancelBtn string
	if m.deleteButtonIdx == 0 {
		deleteBtn = selectedBtn.Render(i18n.T("common.delete"))
		cancelBtn = unselectedBtn.Render(i18n.T("common.cancel"))
	} else {
		deleteBtn = unselectedBtn.Render(i18n.T("common.delete"))
		cancelBtn = selectedBtn.Background(components.Muted).Foreground(lipgloss.Color("#FFFFFF")).Render(i18n.T("common.cancel"))
	}

	fmt.Fprintf(&b, "%s  %s\n\n", deleteBtn, cancelBtn)

	hintStyle := lipgloss.NewStyle().Foreground(components.Muted)
	b.WriteString(hintStyle.Render(fmt.Sprintf("←/→ %s • enter %s • esc %s", i18n.T("help.select"), i18n.T("help.confirm"), i18n.T("help.cancel"))))

	// Wrap with padding
	dialogStyle := lipgloss.NewStyle().Padding(1, 2)
	return dialogStyle.Render(b.String())
}

func (m *CalendarApp) renderEventDetail() string {
	var b strings.Builder

	dayEvents := m.eventsForDate(m.selectedDate)
	if m.selectedIdx >= len(dayEvents) {
		return ""
	}
	event := dayEvents[m.selectedIdx]

	// Build content for the box
	var content strings.Builder

	labelStyle := lipgloss.NewStyle().
		Foreground(components.Muted).
		Width(11)
	valueStyle := lipgloss.NewStyle().
		Foreground(components.Text)

	// Title
	content.WriteString(labelStyle.Render(i18n.T("calendar.field.title")))
	content.WriteString(valueStyle.Bold(true).Render(event.Title))
	content.WriteString("\n")

	// Date
	content.WriteString(labelStyle.Render(i18n.T("calendar.field.date")))
	content.WriteString(valueStyle.Render(event.StartTime.Format("Monday, January 2, 2006")))
	content.WriteString("\n")

	// Time
	var timeStr string
	if event.AllDay {
		timeStr = i18n.T("calendar.all_day")
	} else {
		timeStr = fmt.Sprintf("%s - %s", event.StartTime.Format("3:04 PM"), event.EndTime.Format("3:04 PM"))
	}
	content.WriteString(labelStyle.Render(i18n.T("calendar.field.time")))
	content.WriteString(valueStyle.Render(timeStr))
	content.WriteString("\n")

	// Location (if present)
	if event.Location != "" {
		content.WriteString(labelStyle.Render(i18n.T("calendar.field.location")))
		content.WriteString(valueStyle.Render(event.Location))
		content.WriteString("\n")
	}

	// Calendar
	if event.Calendar != "" {
		content.WriteString(labelStyle.Render(i18n.T("calendar.field.calendar")))
		content.WriteString(valueStyle.Foreground(components.Secondary).Render(event.Calendar))
		content.WriteString("\n")
	}

	// Notes (if present)
	if event.Notes != "" {
		content.WriteString("\n")
		content.WriteString(labelStyle.Render(i18n.T("calendar.field.notes")))
		content.WriteString(valueStyle.Render(event.Notes))
	}

	// Create bordered box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(components.Primary).
		Padding(1, 2).
		Width(50)

	b.WriteString(boxStyle.Render(content.String()))
	b.WriteString("\n\n")

	// Action buttons - highlight selected one
	selectedBtn := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 2)
	unselectedBtn := lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(components.Muted)

	var editBtn, deleteBtn, closeBtn string
	if m.detailButtonIdx == 0 {
		editBtn = selectedBtn.Background(components.Primary).Foreground(lipgloss.Color("#FFFFFF")).Render(i18n.T("common.edit"))
	} else {
		editBtn = unselectedBtn.Render(i18n.T("common.edit"))
	}
	if m.detailButtonIdx == 1 {
		deleteBtn = selectedBtn.Background(components.Danger).Foreground(lipgloss.Color("#FFFFFF")).Render(i18n.T("common.delete"))
	} else {
		deleteBtn = unselectedBtn.Render(i18n.T("common.delete"))
	}
	if m.detailButtonIdx == 2 {
		closeBtn = selectedBtn.Background(components.Muted).Foreground(lipgloss.Color("#FFFFFF")).Render(i18n.T("common.close"))
	} else {
		closeBtn = unselectedBtn.Render(i18n.T("common.close"))
	}

	fmt.Fprintf(&b, "%s  %s  %s\n\n", editBtn, deleteBtn, closeBtn)

	// Help hint
	hintStyle := lipgloss.NewStyle().Foreground(components.Muted)
	b.WriteString(hintStyle.Render(fmt.Sprintf("←/→ %s • enter %s • esc %s", i18n.T("help.select"), i18n.T("help.confirm"), i18n.T("help.close"))))

	// Wrap with padding
	detailStyle := lipgloss.NewStyle().Padding(1, 2)
	return detailStyle.Render(b.String())
}
