package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"maily/internal/calendar"
	"maily/internal/i18n"
	"maily/internal/ui/components"
)

func (m *CalendarApp) renderCalendar() string {
	var b strings.Builder

	// Month header
	monthHeader := lipgloss.NewStyle().
		Bold(true).
		Foreground(components.Primary).
		Padding(0, 1).
		Render(m.selectedDate.Format("January 2006"))

	b.WriteString(monthHeader)
	b.WriteString("\n\n")

	// Weekday headers
	weekdays := []string{
		i18n.T("calendar.weekday.sun"),
		i18n.T("calendar.weekday.mon"),
		i18n.T("calendar.weekday.tue"),
		i18n.T("calendar.weekday.wed"),
		i18n.T("calendar.weekday.thu"),
		i18n.T("calendar.weekday.fri"),
		i18n.T("calendar.weekday.sat"),
	}
	headerStyle := lipgloss.NewStyle().
		Foreground(components.Muted).
		Width(7).
		Align(lipgloss.Center)

	for _, day := range weekdays {
		b.WriteString(headerStyle.Render(day))
	}
	b.WriteString("\n")

	// Calendar grid
	b.WriteString(m.renderMonthGrid())
	b.WriteString("\n")

	// Separator
	separator := lipgloss.NewStyle().
		Foreground(components.Muted).
		Render(strings.Repeat("─", min(m.width, 50)))
	b.WriteString(separator)
	b.WriteString("\n\n")

	// Selected date header
	dateHeader := lipgloss.NewStyle().
		Bold(true).
		Foreground(components.Text).
		Render(m.selectedDate.Format("Mon, Jan 2"))
	b.WriteString(dateHeader)
	b.WriteString("\n\n")

	// Events for selected date
	dayEvents := m.eventsForDate(m.selectedDate)
	if len(dayEvents) == 0 {
		noEvents := lipgloss.NewStyle().
			Foreground(components.Muted).
			Italic(true).
			Render("  " + i18n.T("calendar.no_events"))
		b.WriteString(noEvents)
	} else {
		for i, event := range dayEvents {
			b.WriteString(m.renderEvent(event, i == m.selectedIdx))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Error message if any
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(components.Danger)
		b.WriteString(errStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n")
	}

	// Help bar
	b.WriteString(m.renderHelpBar())

	// Wrap with padding
	calStyle := lipgloss.NewStyle().Padding(1, 2)
	return calStyle.Render(b.String())
}

func (m *CalendarApp) renderMonthGrid() string {
	var b strings.Builder

	year, month, _ := m.selectedDate.Date()
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, m.selectedDate.Location())
	lastDay := firstDay.AddDate(0, 1, -1)

	today := time.Now()
	todayStr := today.Format("2006-01-02")
	selectedStr := m.selectedDate.Format("2006-01-02")

	// Start from the Sunday of the first week
	startDay := firstDay.AddDate(0, 0, -int(firstDay.Weekday()))

	dayStyle := lipgloss.NewStyle().Width(7).Align(lipgloss.Center)
	selectedStyle := dayStyle.Background(components.Primary).Foreground(components.Text)
	todayStyle := dayStyle.Bold(true).Foreground(components.Secondary)
	otherMonthStyle := dayStyle.Foreground(components.Muted)
	hasEventStyle := lipgloss.NewStyle().Foreground(components.Success)

	for week := 0; week < 6; week++ {
		for dow := 0; dow < 7; dow++ {
			day := startDay.AddDate(0, 0, week*7+dow)
			dayStr := day.Format("2006-01-02")

			// Check if this day has events
			hasEvents := false
			for _, e := range m.events {
				if e.StartTime.Format("2006-01-02") == dayStr {
					hasEvents = true
					break
				}
			}

			content := fmt.Sprintf("%2d", day.Day())
			if hasEvents {
				content += hasEventStyle.Render("•")
			} else {
				content += " "
			}

			var style lipgloss.Style
			switch {
			case dayStr == selectedStr:
				style = selectedStyle
			case dayStr == todayStr:
				style = todayStyle
			case day.Month() != month:
				style = otherMonthStyle
			case day.Before(firstDay) || day.After(lastDay):
				style = otherMonthStyle
			default:
				style = dayStyle
			}

			b.WriteString(style.Render(content))
		}
		b.WriteString("\n")

		// Stop if we've passed the last day of the month and completed the week
		if startDay.AddDate(0, 0, (week+1)*7).After(lastDay) && week >= 3 {
			break
		}
	}

	return b.String()
}

func (m *CalendarApp) renderEvent(event calendar.Event, selected bool) string {
	var timeStr string
	if event.AllDay {
		timeStr = i18n.T("calendar.all_day")
	} else {
		timeStr = fmt.Sprintf("%s - %s", event.StartTime.Format("3:04 PM"), event.EndTime.Format("3:04 PM"))
	}

	timeStyle := lipgloss.NewStyle().
		Foreground(components.Muted).
		Width(20)

	titleStyle := lipgloss.NewStyle().Foreground(components.Text)
	calStyle := lipgloss.NewStyle().Foreground(components.Secondary)

	var prefix string
	if selected {
		prefix = lipgloss.NewStyle().Foreground(components.Primary).Render("▸ ")
		titleStyle = titleStyle.Bold(true)
	} else {
		prefix = "  "
	}

	line := prefix + timeStyle.Render(timeStr) + titleStyle.Render(event.Title)
	if event.Calendar != "" {
		line += calStyle.Render(fmt.Sprintf(" [%s]", event.Calendar))
	}

	return line
}

func (m *CalendarApp) renderHelpBar() string {
	helpStyle := lipgloss.NewStyle().Foreground(components.Muted)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Secondary)
	modeStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)

	// Helper for key + label formatting
	key := func(k, label string) string {
		return fmt.Sprintf("%s %s", keyStyle.Render(k), label)
	}

	// Show mode indicator when in navigation mode
	if m.pendingKey != "" {
		modeName := map[string]string{"m": i18n.T("calendar.mode.month"), "y": i18n.T("calendar.mode.year")}[m.pendingKey]
		return fmt.Sprintf("%s  %s", modeStyle.Render("["+modeName+"]"),
			helpStyle.Render(fmt.Sprintf("%s  %s", key("↑↓", i18n.T("help.navigate")), key("esc", i18n.T("calendar.exit_mode")))))
	}

	// Row 1: Navigation
	row1 := []string{
		key("←→", i18n.T("calendar.nav.day")),
		key("↑↓", i18n.T("calendar.nav.week")),
		key("tab", i18n.T("calendar.nav.event")),
		key("m", i18n.T("calendar.nav.month")),
		key("y", i18n.T("calendar.nav.year")),
		key("t", i18n.T("calendar.today")),
	}

	// Row 2: Actions
	row2 := []string{
		key("enter", i18n.T("calendar.action.view")),
		key("n", i18n.T("calendar.action.new")),
		key("e", i18n.T("help.edit")),
		key("d", i18n.T("help.delete")),
		key("q", i18n.T("help.quit")),
	}

	return fmt.Sprintf("%s\n%s", helpStyle.Render(strings.Join(row1, "  ")), helpStyle.Render(strings.Join(row2, "  ")))
}

