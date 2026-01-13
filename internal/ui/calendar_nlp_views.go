package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"maily/internal/ui/components"
	"maily/internal/ui/utils"
)

// NLP Quick-Add render functions

func (m *CalendarApp) renderNLPInput() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))

	b.WriteString(titleStyle.Render("Quick Add Event"))
	b.WriteString("\n\n")
	b.WriteString("    " + m.nlpInput.View())
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("enter confirm • esc cancel"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderNLPParsing() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)

	b.WriteString(titleStyle.Render("Parsing..."))
	b.WriteString("\n\n")
	b.WriteString("    Using AI to parse: \"" + m.nlpInput.Value() + "\"")

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderNLPEdit() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	labelStyle := lipgloss.NewStyle().Width(12).Foreground(components.Muted)
	focusedLabel := lipgloss.NewStyle().Width(12).Foreground(components.Primary).Bold(true)

	b.WriteString(titleStyle.Render("Edit Parsed Event"))
	b.WriteString("\n\n")

	// Title
	if m.nlpEditFocus == 0 {
		b.WriteString("    " + focusedLabel.Render("Title:") + m.nlpEditTitle.View() + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render("Title:") + m.nlpEditTitle.View() + "\n")
	}

	// Date
	if m.nlpEditFocus == 1 {
		b.WriteString("    " + focusedLabel.Render("Date:") + m.nlpEditDate.View())
		b.WriteString(hintStyle.Render("  (↑↓ scroll, ←→ switch)") + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render("Date:") + m.nlpEditDate.View() + "\n")
	}

	// Start time
	if m.nlpEditFocus == 2 {
		b.WriteString("    " + focusedLabel.Render("Start:") + m.nlpEditStart.View())
		b.WriteString(hintStyle.Render("  (↑↓ scroll, ←→ switch)") + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render("Start:") + m.nlpEditStart.View() + "\n")
	}

	// End time
	if m.nlpEditFocus == 3 {
		b.WriteString("    " + focusedLabel.Render("End:") + m.nlpEditEnd.View())
		b.WriteString(hintStyle.Render("  (↑↓ scroll, ←→ switch)") + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render("End:") + m.nlpEditEnd.View() + "\n")
	}

	// Location
	if m.nlpEditFocus == 4 {
		b.WriteString("    " + focusedLabel.Render("Location:") + m.nlpEditLocation.View() + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render("Location:") + m.nlpEditLocation.View() + "\n")
	}

	// Error
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(components.Danger)
		b.WriteString("\n")
		b.WriteString("    " + errStyle.Render(m.err.Error()))
	}

	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("tab next field • enter confirm • esc cancel"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderNLPCalendar() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	itemStyle := lipgloss.NewStyle().PaddingLeft(4)
	cursorStyle := lipgloss.NewStyle().PaddingLeft(4).Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(components.Primary)

	// Show parsed event
	b.WriteString(m.renderNLPEventBox())
	b.WriteString("\n")

	b.WriteString(titleStyle.Render("Select Calendar"))
	b.WriteString("\n\n")

	for i, cal := range m.calendars {
		if i == m.nlpCalendarIdx {
			b.WriteString(cursorStyle.Render("> " + cal.Title))
		} else {
			b.WriteString(itemStyle.Render("  " + cal.Title))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/k up • ↓/j down • enter select • esc cancel"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderNLPReminder() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	itemStyle := lipgloss.NewStyle().PaddingLeft(4)
	cursorStyle := lipgloss.NewStyle().PaddingLeft(4).Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(components.Primary)

	// Show parsed event
	b.WriteString(m.renderNLPEventBox())
	b.WriteString("\n")

	b.WriteString(titleStyle.Render("Reminder"))
	b.WriteString("\n\n")

	reminderOptions := []string{
		"No reminder",
		"5 minutes before",
		"10 minutes before",
		"15 minutes before",
		"30 minutes before",
		"1 hour before",
	}

	for i, opt := range reminderOptions {
		if i == m.nlpReminderIdx {
			b.WriteString(cursorStyle.Render("> " + opt))
		} else {
			b.WriteString(itemStyle.Render("  " + opt))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/k up • ↓/j down • enter select • esc cancel"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderNLPConfirm() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))

	b.WriteString(titleStyle.Render("Confirm Event"))
	b.WriteString("\n\n")

	// Event details box
	b.WriteString("  ┌────────────────────────────────────────────────┐\n")
	b.WriteString("  │  Title:    " + utils.PadRight(utils.TruncateStr(m.nlpParsed.Title, 35), 35) + "│\n")
	b.WriteString("  │  Date:     " + utils.PadRight(m.nlpStartTime.Format("Monday, Jan 2, 2006"), 35) + "│\n")
	b.WriteString("  │  Time:     " + utils.PadRight(fmt.Sprintf("%s - %s", m.nlpStartTime.Format("3:04 PM"), m.nlpEndTime.Format("3:04 PM")), 35) + "│\n")
	if m.nlpParsed.Location != "" {
		b.WriteString("  │  Location: " + utils.PadRight(utils.TruncateStr(m.nlpParsed.Location, 35), 35) + "│\n")
	}
	calName := "Default"
	if len(m.calendars) > 0 && m.nlpCalendarIdx < len(m.calendars) {
		calName = m.calendars[m.nlpCalendarIdx].Title
	}
	b.WriteString("  │  Calendar: " + utils.PadRight(utils.TruncateStr(calName, 35), 35) + "│\n")
	reminderStr := "None"
	if mins := m.getNLPReminderMinutes(); mins > 0 {
		if mins == 60 {
			reminderStr = "1 hour before"
		} else {
			reminderStr = fmt.Sprintf("%d minutes before", mins)
		}
	}
	b.WriteString("  │  Reminder: " + utils.PadRight(reminderStr, 35) + "│\n")
	b.WriteString("  └────────────────────────────────────────────────┘\n")

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("enter create • esc cancel"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderNLPEventBox() string {
	var b strings.Builder

	b.WriteString("  ┌─ Parsed Event ─────────────────────────────────┐\n")
	b.WriteString("  │  Title:    " + utils.PadRight(utils.TruncateStr(m.nlpParsed.Title, 37), 37) + "│\n")
	b.WriteString("  │  Date:     " + utils.PadRight(m.nlpStartTime.Format("Monday, Jan 2, 2006"), 37) + "│\n")
	b.WriteString("  │  Time:     " + utils.PadRight(fmt.Sprintf("%s - %s", m.nlpStartTime.Format("3:04 PM"), m.nlpEndTime.Format("3:04 PM")), 37) + "│\n")
	if m.nlpParsed.Location != "" {
		b.WriteString("  │  Location: " + utils.PadRight(utils.TruncateStr(m.nlpParsed.Location, 37), 37) + "│\n")
	}
	b.WriteString("  └────────────────────────────────────────────────┘")

	return b.String()
}

// Interactive Form render functions (fallback when no AI CLI available)

func (m *CalendarApp) renderFormTitle() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	stepStyle := lipgloss.NewStyle().Foreground(components.Muted)

	b.WriteString(titleStyle.Render("New Event"))
	b.WriteString("  ")
	b.WriteString(stepStyle.Render("Step 1 of 4"))
	b.WriteString("\n\n")

	b.WriteString("  What's the event?\n\n")
	b.WriteString("    " + m.formTitleInput.View())
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("enter next • esc cancel"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderFormDateTime() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	stepStyle := lipgloss.NewStyle().Foreground(components.Muted)
	labelStyle := lipgloss.NewStyle().Width(12).Foreground(components.Muted)
	focusedLabel := lipgloss.NewStyle().Width(12).Foreground(components.Primary).Bold(true)

	b.WriteString(titleStyle.Render("New Event"))
	b.WriteString("  ")
	b.WriteString(stepStyle.Render("Step 2 of 4"))
	b.WriteString("\n\n")

	// Show title
	b.WriteString("  ┌─ Event ──────────────────────────────────────────┐\n")
	b.WriteString("  │  Title: " + utils.PadRight(utils.TruncateStr(m.formTitleInput.Value(), 41), 41) + "│\n")
	b.WriteString("  └────────────────────────────────────────────────────┘\n\n")

	b.WriteString("  When is it?\n\n")

	// Date
	if m.formFocusField == 0 {
		b.WriteString("    " + focusedLabel.Render("Date:") + m.formDateInput.View())
		b.WriteString(hintStyle.Render("  (↑↓ scroll, ←→ switch)") + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render("Date:") + m.formDateInput.View() + "\n")
	}

	// Start time
	if m.formFocusField == 1 {
		b.WriteString("    " + focusedLabel.Render("Start:") + m.formStartInput.View())
		b.WriteString(hintStyle.Render("  (↑↓ scroll, ←→ switch)") + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render("Start:") + m.formStartInput.View() + "\n")
	}

	// End time
	if m.formFocusField == 2 {
		b.WriteString("    " + focusedLabel.Render("End:") + m.formEndInput.View())
		b.WriteString(hintStyle.Render("  (↑↓ scroll, ←→ switch)") + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render("End:") + m.formEndInput.View() + "\n")
	}

	// Location
	if m.formFocusField == 3 {
		b.WriteString("    " + focusedLabel.Render("Location:") + m.formLocationInput.View() + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render("Location:") + m.formLocationInput.View() + "\n")
	}

	// Error
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(components.Danger)
		b.WriteString("\n")
		b.WriteString("    " + errStyle.Render(m.err.Error()))
	}

	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("tab next field • enter next step • esc cancel"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderFormCalendar() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	stepStyle := lipgloss.NewStyle().Foreground(components.Muted)
	itemStyle := lipgloss.NewStyle().PaddingLeft(4)
	cursorStyle := lipgloss.NewStyle().PaddingLeft(4).Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(components.Primary)

	b.WriteString(titleStyle.Render("New Event"))
	b.WriteString("  ")
	b.WriteString(stepStyle.Render("Step 3 of 4"))
	b.WriteString("\n\n")

	// Show event summary
	b.WriteString(m.renderFormEventBox())
	b.WriteString("\n")

	b.WriteString(titleStyle.Render("Select Calendar"))
	b.WriteString("\n\n")

	for i, cal := range m.calendars {
		if i == m.formCalendarIdx {
			b.WriteString(cursorStyle.Render("> " + cal.Title))
		} else {
			b.WriteString(itemStyle.Render("  " + cal.Title))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/k up • ↓/j down • enter select • esc cancel"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderFormReminder() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	stepStyle := lipgloss.NewStyle().Foreground(components.Muted)
	itemStyle := lipgloss.NewStyle().PaddingLeft(4)
	cursorStyle := lipgloss.NewStyle().PaddingLeft(4).Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(components.Primary)

	b.WriteString(titleStyle.Render("New Event"))
	b.WriteString("  ")
	b.WriteString(stepStyle.Render("Step 4 of 4"))
	b.WriteString("\n\n")

	// Show event summary
	b.WriteString(m.renderFormEventBox())
	b.WriteString("\n")

	b.WriteString(titleStyle.Render("Reminder"))
	b.WriteString("\n\n")

	reminderOptions := []string{
		"No reminder",
		"5 minutes before",
		"10 minutes before",
		"15 minutes before",
		"30 minutes before",
		"1 hour before",
	}

	for i, opt := range reminderOptions {
		if i == m.formReminderIdx {
			b.WriteString(cursorStyle.Render("> " + opt))
		} else {
			b.WriteString(itemStyle.Render("  " + opt))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/k up • ↓/j down • enter select • esc cancel"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderFormConfirm() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))

	b.WriteString(titleStyle.Render("Confirm Event"))
	b.WriteString("\n\n")

	startTime := m.getFormStartTime()
	endTime := m.getFormEndTime()

	// Event details box
	b.WriteString("  ┌────────────────────────────────────────────────┐\n")
	b.WriteString("  │  Title:    " + utils.PadRight(utils.TruncateStr(m.formTitleInput.Value(), 35), 35) + "│\n")
	b.WriteString("  │  Date:     " + utils.PadRight(startTime.Format("Monday, Jan 2, 2006"), 35) + "│\n")
	b.WriteString("  │  Time:     " + utils.PadRight(fmt.Sprintf("%s - %s", startTime.Format("3:04 PM"), endTime.Format("3:04 PM")), 35) + "│\n")
	if m.formLocationInput.Value() != "" {
		b.WriteString("  │  Location: " + utils.PadRight(utils.TruncateStr(m.formLocationInput.Value(), 35), 35) + "│\n")
	}
	calName := "Default"
	if len(m.calendars) > 0 && m.formCalendarIdx < len(m.calendars) {
		calName = m.calendars[m.formCalendarIdx].Title
	}
	b.WriteString("  │  Calendar: " + utils.PadRight(utils.TruncateStr(calName, 35), 35) + "│\n")
	reminderStr := "None"
	if mins := m.getFormReminderMinutes(); mins > 0 {
		if mins == 60 {
			reminderStr = "1 hour before"
		} else {
			reminderStr = fmt.Sprintf("%d minutes before", mins)
		}
	}
	b.WriteString("  │  Reminder: " + utils.PadRight(reminderStr, 35) + "│\n")
	b.WriteString("  └────────────────────────────────────────────────┘\n")

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("enter create • esc cancel"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderFormEventBox() string {
	var b strings.Builder

	startTime := m.getFormStartTime()
	endTime := m.getFormEndTime()

	b.WriteString("  ┌─ Event ──────────────────────────────────────────┐\n")
	b.WriteString("  │  Title:    " + utils.PadRight(utils.TruncateStr(m.formTitleInput.Value(), 39), 39) + "│\n")
	b.WriteString("  │  Date:     " + utils.PadRight(startTime.Format("Monday, Jan 2, 2006"), 39) + "│\n")
	b.WriteString("  │  Time:     " + utils.PadRight(fmt.Sprintf("%s - %s", startTime.Format("3:04 PM"), endTime.Format("3:04 PM")), 39) + "│\n")
	if m.formLocationInput.Value() != "" {
		b.WriteString("  │  Location: " + utils.PadRight(utils.TruncateStr(m.formLocationInput.Value(), 39), 39) + "│\n")
	}
	b.WriteString("  └────────────────────────────────────────────────────┘")

	return b.String()
}
