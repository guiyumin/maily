package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"maily/internal/i18n"
	"maily/internal/ui/components"
	"maily/internal/ui/utils"
)

// NLP Quick-Add render functions

func (m *CalendarApp) renderNLPInput() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))

	b.WriteString(titleStyle.Render(i18n.T("calendar.quick_add")))
	b.WriteString("\n\n")
	b.WriteString(m.nlpInput.View())
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render(fmt.Sprintf("ctrl+enter %s • esc %s", i18n.T("help.confirm"), i18n.T("help.cancel"))))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderNLPParsing() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)

	b.WriteString(titleStyle.Render(i18n.T("calendar.parsing")))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("    %s", i18n.T("calendar.parsing_input", map[string]any{"Input": m.nlpInput.Value()})))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderNLPEdit() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	labelStyle := lipgloss.NewStyle().Width(12).Foreground(components.Muted)
	focusedLabel := lipgloss.NewStyle().Width(12).Foreground(components.Primary).Bold(true)

	b.WriteString(titleStyle.Render(i18n.T("calendar.edit_parsed")))
	b.WriteString("\n\n")

	// Helper for form field rows
	field := func(focused bool, label, value string, showHint bool) string {
		style := labelStyle
		if focused {
			style = focusedLabel
		}
		row := fmt.Sprintf("    %s%s", style.Render(label), value)
		if focused && showHint {
			row += hintStyle.Render(fmt.Sprintf("  %s", i18n.T("calendar.picker_hint")))
		}
		return row + "\n"
	}

	// Title
	b.WriteString(field(m.nlpEditFocus == 0, i18n.T("calendar.field.title"), m.nlpEditTitle.View(), false))

	// Date
	b.WriteString(field(m.nlpEditFocus == 1, i18n.T("calendar.field.date"), m.nlpEditDate.View(), true))

	// Start time
	b.WriteString(field(m.nlpEditFocus == 2, i18n.T("calendar.field.start"), m.nlpEditStart.View(), true))

	// End time
	b.WriteString(field(m.nlpEditFocus == 3, i18n.T("calendar.field.end"), m.nlpEditEnd.View(), true))

	// Location
	b.WriteString(field(m.nlpEditFocus == 4, i18n.T("calendar.field.location"), m.nlpEditLocation.View(), false))

	// Notes
	b.WriteString("\n")
	b.WriteString(field(m.nlpEditFocus == 5, i18n.T("calendar.field.notes"), "", false))
	b.WriteString(fmt.Sprintf("    %s\n", m.nlpEditNotes.View()))

	// Error
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(components.Danger)
		b.WriteString(fmt.Sprintf("\n    %s", errStyle.Render(m.err.Error())))
	}

	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render(fmt.Sprintf("tab %s • enter %s • esc %s", i18n.T("help.next_field"), i18n.T("help.confirm"), i18n.T("help.cancel"))))

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

	b.WriteString(titleStyle.Render(i18n.T("calendar.select_calendar")))
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
	b.WriteString(hintStyle.Render(i18n.T("calendar.select_hint")))

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

	b.WriteString(titleStyle.Render(i18n.T("calendar.reminder")))
	b.WriteString("\n\n")

	reminderOptions := []string{
		i18n.T("calendar.reminder.none"),
		i18n.T("calendar.reminder.5min"),
		i18n.T("calendar.reminder.10min"),
		i18n.T("calendar.reminder.15min"),
		i18n.T("calendar.reminder.30min"),
		i18n.T("calendar.reminder.1hour"),
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
	b.WriteString(hintStyle.Render(i18n.T("calendar.select_hint")))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderNLPConfirm() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))

	b.WriteString(titleStyle.Render(i18n.T("calendar.confirm_event")))
	b.WriteString("\n\n")

	// Helper for box rows
	boxRow := func(label, value string, width int) string {
		return fmt.Sprintf("  │  %s %s│\n", label, utils.PadRight(value, width))
	}

	// Event details box
	b.WriteString("  ┌────────────────────────────────────────────────┐\n")
	b.WriteString(boxRow(i18n.T("calendar.field.title"), utils.TruncateStr(m.nlpParsed.Title, 35), 35))
	b.WriteString(boxRow(i18n.T("calendar.field.date")+" ", m.nlpStartTime.Format("Monday, Jan 2, 2006"), 35))
	b.WriteString(boxRow(i18n.T("calendar.field.time")+" ", fmt.Sprintf("%s - %s", m.nlpStartTime.Format("3:04 PM"), m.nlpEndTime.Format("3:04 PM")), 35))
	if m.nlpParsed.Location != "" {
		b.WriteString(boxRow(i18n.T("calendar.field.location"), utils.TruncateStr(m.nlpParsed.Location, 35), 35))
	}
	calName := i18n.T("calendar.default")
	if len(m.calendars) > 0 && m.nlpCalendarIdx < len(m.calendars) {
		calName = m.calendars[m.nlpCalendarIdx].Title
	}
	b.WriteString(boxRow(i18n.T("calendar.field.calendar"), utils.TruncateStr(calName, 35), 35))
	reminderStr := i18n.T("calendar.reminder.none")
	if mins := m.getNLPReminderMinutes(); mins > 0 {
		if mins == 60 {
			reminderStr = i18n.T("calendar.reminder.1hour")
		} else {
			reminderStr = i18n.T("calendar.reminder.minutes", map[string]any{"Minutes": mins})
		}
	}
	b.WriteString(boxRow(i18n.T("calendar.field.reminder"), reminderStr, 35))
	b.WriteString("  └────────────────────────────────────────────────┘\n")

	b.WriteString("\n")
	b.WriteString(hintStyle.Render(fmt.Sprintf("enter %s • esc %s", i18n.T("calendar.create"), i18n.T("help.cancel"))))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderNLPEventBox() string {
	var b strings.Builder

	// Helper for box rows
	boxRow := func(label, value string) string {
		return fmt.Sprintf("  │  %s %s│\n", label, utils.PadRight(value, 37))
	}

	fmt.Fprintf(&b, "  ┌─ %s ─────────────────────────────────┐\n", i18n.T("calendar.parsed_event"))
	b.WriteString(boxRow(i18n.T("calendar.field.title"), utils.TruncateStr(m.nlpParsed.Title, 37)))
	b.WriteString(boxRow(i18n.T("calendar.field.date")+" ", m.nlpStartTime.Format("Monday, Jan 2, 2006")))
	b.WriteString(boxRow(i18n.T("calendar.field.time")+" ", fmt.Sprintf("%s - %s", m.nlpStartTime.Format("3:04 PM"), m.nlpEndTime.Format("3:04 PM"))))
	if m.nlpParsed.Location != "" {
		b.WriteString(boxRow(i18n.T("calendar.field.location"), utils.TruncateStr(m.nlpParsed.Location, 37)))
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

	fmt.Fprintf(&b, "%s  %s\n\n", titleStyle.Render(i18n.T("calendar.new_event")), stepStyle.Render(i18n.T("calendar.step", map[string]any{"Current": 1, "Total": 4})))
	fmt.Fprintf(&b, "  %s\n\n", i18n.T("calendar.what_event"))
	fmt.Fprintf(&b, "    %s\n\n", m.formTitleInput.View())
	b.WriteString(hintStyle.Render(fmt.Sprintf("enter %s • esc %s", i18n.T("calendar.next"), i18n.T("help.cancel"))))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderFormDateTime() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	stepStyle := lipgloss.NewStyle().Foreground(components.Muted)
	labelStyle := lipgloss.NewStyle().Width(12).Foreground(components.Muted)
	focusedLabel := lipgloss.NewStyle().Width(12).Foreground(components.Primary).Bold(true)

	// Helper for form field rows
	field := func(focused bool, label, value string, showHint bool) string {
		style := labelStyle
		if focused {
			style = focusedLabel
		}
		row := fmt.Sprintf("    %s%s", style.Render(label), value)
		if focused && showHint {
			row += hintStyle.Render(fmt.Sprintf("  %s", i18n.T("calendar.picker_hint")))
		}
		return row + "\n"
	}

	fmt.Fprintf(&b, "%s  %s\n\n", titleStyle.Render(i18n.T("calendar.new_event")), stepStyle.Render(i18n.T("calendar.step", map[string]any{"Current": 2, "Total": 4})))

	// Show title
	fmt.Fprintf(&b, "  ┌─ %s ──────────────────────────────────────────┐\n", i18n.T("calendar.event"))
	fmt.Fprintf(&b, "  │  %s %s│\n", i18n.T("calendar.field.title"), utils.PadRight(utils.TruncateStr(m.formTitleInput.Value(), 41), 41))
	b.WriteString("  └────────────────────────────────────────────────────┘\n\n")

	fmt.Fprintf(&b, "  %s\n\n", i18n.T("calendar.when_event"))

	// Date, Start, End, Location fields
	b.WriteString(field(m.formFocusField == 0, i18n.T("calendar.field.date"), m.formDateInput.View(), true))
	b.WriteString(field(m.formFocusField == 1, i18n.T("calendar.field.start"), m.formStartInput.View(), true))
	b.WriteString(field(m.formFocusField == 2, i18n.T("calendar.field.end"), m.formEndInput.View(), true))
	b.WriteString(field(m.formFocusField == 3, i18n.T("calendar.field.location"), m.formLocationInput.View(), false))

	// Notes
	b.WriteString("\n")
	b.WriteString(field(m.formFocusField == 4, i18n.T("calendar.field.notes"), "", false))
	fmt.Fprintf(&b, "    %s\n", m.formNotesInput.View())

	// Error
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(components.Danger)
		fmt.Fprintf(&b, "\n    %s", errStyle.Render(m.err.Error()))
	}

	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render(fmt.Sprintf("tab %s • enter %s • esc %s", i18n.T("help.next_field"), i18n.T("calendar.next_step"), i18n.T("help.cancel"))))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderFormCalendar() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	stepStyle := lipgloss.NewStyle().Foreground(components.Muted)
	itemStyle := lipgloss.NewStyle().PaddingLeft(4)
	cursorStyle := lipgloss.NewStyle().PaddingLeft(4).Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(components.Primary)

	b.WriteString(titleStyle.Render(i18n.T("calendar.new_event")))
	b.WriteString("  ")
	b.WriteString(stepStyle.Render(i18n.T("calendar.step", map[string]any{"Current": 3, "Total": 4})))
	b.WriteString("\n\n")

	// Show event summary
	b.WriteString(m.renderFormEventBox())
	b.WriteString("\n")

	b.WriteString(titleStyle.Render(i18n.T("calendar.select_calendar")))
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
	b.WriteString(hintStyle.Render(i18n.T("calendar.select_hint")))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderFormReminder() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	stepStyle := lipgloss.NewStyle().Foreground(components.Muted)
	itemStyle := lipgloss.NewStyle().PaddingLeft(4)
	cursorStyle := lipgloss.NewStyle().PaddingLeft(4).Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(components.Primary)

	b.WriteString(titleStyle.Render(i18n.T("calendar.new_event")))
	b.WriteString("  ")
	b.WriteString(stepStyle.Render(i18n.T("calendar.step", map[string]any{"Current": 4, "Total": 4})))
	b.WriteString("\n\n")

	// Show event summary
	b.WriteString(m.renderFormEventBox())
	b.WriteString("\n")

	b.WriteString(titleStyle.Render(i18n.T("calendar.reminder")))
	b.WriteString("\n\n")

	reminderOptions := []string{
		i18n.T("calendar.reminder.none"),
		i18n.T("calendar.reminder.5min"),
		i18n.T("calendar.reminder.10min"),
		i18n.T("calendar.reminder.15min"),
		i18n.T("calendar.reminder.30min"),
		i18n.T("calendar.reminder.1hour"),
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
	b.WriteString(hintStyle.Render(i18n.T("calendar.select_hint")))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderFormConfirm() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))

	fmt.Fprintf(&b, "%s\n\n", titleStyle.Render(i18n.T("calendar.confirm_event")))

	startTime := m.getFormStartTime()
	endTime := m.getFormEndTime()

	// Helper for box rows
	boxRow := func(label, value string) string {
		return fmt.Sprintf("  │  %s %s│\n", label, utils.PadRight(value, 35))
	}

	// Event details box
	b.WriteString("  ┌────────────────────────────────────────────────┐\n")
	b.WriteString(boxRow(i18n.T("calendar.field.title"), utils.TruncateStr(m.formTitleInput.Value(), 35)))
	b.WriteString(boxRow(i18n.T("calendar.field.date")+" ", startTime.Format("Monday, Jan 2, 2006")))
	b.WriteString(boxRow(i18n.T("calendar.field.time")+" ", fmt.Sprintf("%s - %s", startTime.Format("3:04 PM"), endTime.Format("3:04 PM"))))
	if m.formLocationInput.Value() != "" {
		b.WriteString(boxRow(i18n.T("calendar.field.location"), utils.TruncateStr(m.formLocationInput.Value(), 35)))
	}
	calName := i18n.T("calendar.default")
	if len(m.calendars) > 0 && m.formCalendarIdx < len(m.calendars) {
		calName = m.calendars[m.formCalendarIdx].Title
	}
	b.WriteString(boxRow(i18n.T("calendar.field.calendar"), utils.TruncateStr(calName, 35)))
	reminderStr := i18n.T("calendar.reminder.none")
	if mins := m.getFormReminderMinutes(); mins > 0 {
		if mins == 60 {
			reminderStr = i18n.T("calendar.reminder.1hour")
		} else {
			reminderStr = i18n.T("calendar.reminder.minutes", map[string]any{"Minutes": mins})
		}
	}
	b.WriteString(boxRow(i18n.T("calendar.field.reminder"), reminderStr))
	b.WriteString("  └────────────────────────────────────────────────┘\n")

	b.WriteString("\n")
	b.WriteString(hintStyle.Render(fmt.Sprintf("enter %s • esc %s", i18n.T("calendar.create"), i18n.T("help.cancel"))))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderFormEventBox() string {
	var b strings.Builder

	startTime := m.getFormStartTime()
	endTime := m.getFormEndTime()

	// Helper for box rows
	boxRow := func(label, value string) string {
		return fmt.Sprintf("  │  %s %s│\n", label, utils.PadRight(value, 39))
	}

	fmt.Fprintf(&b, "  ┌─ %s ──────────────────────────────────────────┐\n", i18n.T("calendar.event"))
	b.WriteString(boxRow(i18n.T("calendar.field.title"), utils.TruncateStr(m.formTitleInput.Value(), 39)))
	b.WriteString(boxRow(i18n.T("calendar.field.date")+" ", startTime.Format("Monday, Jan 2, 2006")))
	b.WriteString(boxRow(i18n.T("calendar.field.time")+" ", fmt.Sprintf("%s - %s", startTime.Format("3:04 PM"), endTime.Format("3:04 PM"))))
	if m.formLocationInput.Value() != "" {
		b.WriteString(boxRow(i18n.T("calendar.field.location"), utils.TruncateStr(m.formLocationInput.Value(), 39)))
	}
	b.WriteString("  └────────────────────────────────────────────────────┘")

	return b.String()
}
