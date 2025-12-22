package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"maily/internal/calendar"
	"maily/internal/ui/components"
)

// View states
type calendarView int

const (
	viewCalendar calendarView = iota
	viewAddEvent
	viewEditEvent
	viewDeleteConfirm
)

// CalendarApp is the main calendar TUI model
type CalendarApp struct {
	client       calendar.Client
	width        int
	height       int
	selectedDate time.Time
	events       []calendar.Event
	calendars    []calendar.Calendar
	selectedIdx  int // selected event index in the list
	view         calendarView
	err          error

	// Form fields for add/edit
	form         eventForm
	formFocusIdx int
}

type eventForm struct {
	title    textinput.Model
	date     textinput.Model
	start    textinput.Model
	end      textinput.Model
	location textinput.Model
	calendar int // index into calendars slice
	editID   string
}

// Messages
type eventsLoadedMsg struct {
	events []calendar.Event
}

type calendarsLoadedMsg struct {
	calendars []calendar.Calendar
}

type eventCreatedMsg struct {
	id string
}

type eventDeletedMsg struct{}

type errMsg struct {
	err error
}

// NewCalendarApp creates a new calendar TUI
func NewCalendarApp(client calendar.Client) *CalendarApp {
	return &CalendarApp{
		client:       client,
		selectedDate: time.Now(),
		view:         viewCalendar,
	}
}

func (m *CalendarApp) Init() tea.Cmd {
	return tea.Batch(
		m.loadEvents(),
		m.loadCalendars(),
	)
}

func (m *CalendarApp) loadEvents() tea.Cmd {
	return func() tea.Msg {
		// Load events for current month + buffer
		start := time.Date(m.selectedDate.Year(), m.selectedDate.Month(), 1, 0, 0, 0, 0, m.selectedDate.Location())
		end := start.AddDate(0, 1, 7) // Current month + 1 week into next
		start = start.AddDate(0, 0, -7) // 1 week before month start

		events, err := m.client.ListEvents(start, end)
		if err != nil {
			return errMsg{err}
		}
		return eventsLoadedMsg{events}
	}
}

func (m *CalendarApp) loadCalendars() tea.Cmd {
	return func() tea.Msg {
		calendars, err := m.client.ListCalendars()
		if err != nil {
			return errMsg{err}
		}
		return calendarsLoadedMsg{calendars}
	}
}

func (m *CalendarApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case eventsLoadedMsg:
		m.events = msg.events
		return m, nil

	case calendarsLoadedMsg:
		m.calendars = msg.calendars
		return m, nil

	case eventCreatedMsg:
		m.view = viewCalendar
		return m, m.loadEvents()

	case eventDeletedMsg:
		m.view = viewCalendar
		if m.selectedIdx >= len(m.events) {
			m.selectedIdx = len(m.events) - 1
		}
		if m.selectedIdx < 0 {
			m.selectedIdx = 0
		}
		return m, m.loadEvents()

	case errMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	// Update form inputs if in form view
	if m.view == viewAddEvent || m.view == viewEditEvent {
		return m.updateForm(msg)
	}

	return m, nil
}

func (m *CalendarApp) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.view {
	case viewCalendar:
		return m.handleCalendarKeys(msg)
	case viewAddEvent, viewEditEvent:
		return m.handleFormKeys(msg)
	case viewDeleteConfirm:
		return m.handleDeleteKeys(msg)
	}
	return m, nil
}

func (m *CalendarApp) handleCalendarKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Left):
		m.selectedDate = m.selectedDate.AddDate(0, 0, -1)
		m.selectedIdx = 0
		return m, nil

	case key.Matches(msg, keys.Right):
		m.selectedDate = m.selectedDate.AddDate(0, 0, 1)
		m.selectedIdx = 0
		return m, nil

	case key.Matches(msg, keys.Up):
		// Move up in event list, or up a week if at top
		dayEvents := m.eventsForDate(m.selectedDate)
		if m.selectedIdx > 0 {
			m.selectedIdx--
		} else {
			m.selectedDate = m.selectedDate.AddDate(0, 0, -7)
			m.selectedIdx = 0
		}
		_ = dayEvents
		return m, nil

	case key.Matches(msg, keys.Down):
		// Move down in event list, or down a week if at bottom
		dayEvents := m.eventsForDate(m.selectedDate)
		if m.selectedIdx < len(dayEvents)-1 {
			m.selectedIdx++
		} else {
			m.selectedDate = m.selectedDate.AddDate(0, 0, 7)
			m.selectedIdx = 0
		}
		return m, nil

	case key.Matches(msg, keys.PrevMonth):
		m.selectedDate = m.selectedDate.AddDate(0, -1, 0)
		m.selectedIdx = 0
		return m, m.loadEvents()

	case key.Matches(msg, keys.NextMonth):
		m.selectedDate = m.selectedDate.AddDate(0, 1, 0)
		m.selectedIdx = 0
		return m, m.loadEvents()

	case key.Matches(msg, keys.Today):
		m.selectedDate = time.Now()
		m.selectedIdx = 0
		return m, m.loadEvents()

	case key.Matches(msg, keys.Add):
		m.initAddForm()
		m.view = viewAddEvent
		return m, nil

	case key.Matches(msg, keys.Edit):
		dayEvents := m.eventsForDate(m.selectedDate)
		if len(dayEvents) > 0 && m.selectedIdx < len(dayEvents) {
			m.initEditForm(dayEvents[m.selectedIdx])
			m.view = viewEditEvent
		}
		return m, nil

	case key.Matches(msg, keys.Delete):
		dayEvents := m.eventsForDate(m.selectedDate)
		if len(dayEvents) > 0 && m.selectedIdx < len(dayEvents) {
			m.view = viewDeleteConfirm
		}
		return m, nil
	}

	return m, nil
}

func (m *CalendarApp) handleFormKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewCalendar
		return m, nil

	case "tab", "down":
		m.formFocusIdx = (m.formFocusIdx + 1) % 6
		m.updateFormFocus()
		return m, nil

	case "shift+tab", "up":
		m.formFocusIdx = (m.formFocusIdx + 5) % 6
		m.updateFormFocus()
		return m, nil

	case "enter":
		if m.formFocusIdx == 5 { // Calendar selector
			m.form.calendar = (m.form.calendar + 1) % len(m.calendars)
			return m, nil
		}
		// Save event
		return m, m.saveEvent()

	case "ctrl+s":
		return m, m.saveEvent()
	}

	return m, nil
}

func (m *CalendarApp) handleDeleteKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		dayEvents := m.eventsForDate(m.selectedDate)
		if m.selectedIdx < len(dayEvents) {
			return m, m.deleteEvent(dayEvents[m.selectedIdx].ID)
		}
		m.view = viewCalendar
		return m, nil

	case "n", "N", "esc", "q":
		m.view = viewCalendar
		return m, nil
	}

	return m, nil
}

func (m *CalendarApp) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.form.title, cmd = m.form.title.Update(msg)
	cmds = append(cmds, cmd)

	m.form.date, cmd = m.form.date.Update(msg)
	cmds = append(cmds, cmd)

	m.form.start, cmd = m.form.start.Update(msg)
	cmds = append(cmds, cmd)

	m.form.end, cmd = m.form.end.Update(msg)
	cmds = append(cmds, cmd)

	m.form.location, cmd = m.form.location.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *CalendarApp) initAddForm() {
	m.form = eventForm{
		title:    textinput.New(),
		date:     textinput.New(),
		start:    textinput.New(),
		end:      textinput.New(),
		location: textinput.New(),
		calendar: 0,
	}

	m.form.title.Placeholder = "Event title"
	m.form.title.Focus()

	m.form.date.Placeholder = "YYYY-MM-DD"
	m.form.date.SetValue(m.selectedDate.Format("2006-01-02"))

	m.form.start.Placeholder = "HH:MM"
	m.form.start.SetValue("09:00")

	m.form.end.Placeholder = "HH:MM"
	m.form.end.SetValue("10:00")

	m.form.location.Placeholder = "Location (optional)"

	m.formFocusIdx = 0
}

func (m *CalendarApp) initEditForm(event calendar.Event) {
	m.form = eventForm{
		title:    textinput.New(),
		date:     textinput.New(),
		start:    textinput.New(),
		end:      textinput.New(),
		location: textinput.New(),
		editID:   event.ID,
	}

	m.form.title.SetValue(event.Title)
	m.form.title.Focus()

	m.form.date.SetValue(event.StartTime.Format("2006-01-02"))
	m.form.start.SetValue(event.StartTime.Format("15:04"))
	m.form.end.SetValue(event.EndTime.Format("15:04"))
	m.form.location.SetValue(event.Location)

	// Find calendar index
	for i, cal := range m.calendars {
		if cal.Title == event.Calendar {
			m.form.calendar = i
			break
		}
	}

	m.formFocusIdx = 0
}

func (m *CalendarApp) updateFormFocus() {
	m.form.title.Blur()
	m.form.date.Blur()
	m.form.start.Blur()
	m.form.end.Blur()
	m.form.location.Blur()

	switch m.formFocusIdx {
	case 0:
		m.form.title.Focus()
	case 1:
		m.form.date.Focus()
	case 2:
		m.form.start.Focus()
	case 3:
		m.form.end.Focus()
	case 4:
		m.form.location.Focus()
	}
}

func (m *CalendarApp) saveEvent() tea.Cmd {
	return func() tea.Msg {
		date, err := time.Parse("2006-01-02", m.form.date.Value())
		if err != nil {
			return errMsg{fmt.Errorf("invalid date: %v", err)}
		}

		startTime, err := time.Parse("15:04", m.form.start.Value())
		if err != nil {
			return errMsg{fmt.Errorf("invalid start time: %v", err)}
		}

		endTime, err := time.Parse("15:04", m.form.end.Value())
		if err != nil {
			return errMsg{fmt.Errorf("invalid end time: %v", err)}
		}

		start := time.Date(date.Year(), date.Month(), date.Day(),
			startTime.Hour(), startTime.Minute(), 0, 0, time.Local)
		end := time.Date(date.Year(), date.Month(), date.Day(),
			endTime.Hour(), endTime.Minute(), 0, 0, time.Local)

		var calendarID string
		if len(m.calendars) > 0 && m.form.calendar < len(m.calendars) {
			calendarID = m.calendars[m.form.calendar].ID
		}

		event := calendar.Event{
			Title:     m.form.title.Value(),
			StartTime: start,
			EndTime:   end,
			Location:  m.form.location.Value(),
			Calendar:  calendarID,
		}

		// If editing, delete old event first (EventKit doesn't have update)
		if m.form.editID != "" {
			_ = m.client.DeleteEvent(m.form.editID)
		}

		id, err := m.client.CreateEvent(event)
		if err != nil {
			return errMsg{err}
		}

		return eventCreatedMsg{id}
	}
}

func (m *CalendarApp) deleteEvent(id string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeleteEvent(id)
		if err != nil {
			return errMsg{err}
		}
		return eventDeletedMsg{}
	}
}

func (m *CalendarApp) eventsForDate(date time.Time) []calendar.Event {
	var result []calendar.Event
	dateStr := date.Format("2006-01-02")

	for _, e := range m.events {
		if e.StartTime.Format("2006-01-02") == dateStr {
			result = append(result, e)
		}
	}
	return result
}

func (m *CalendarApp) View() string {
	switch m.view {
	case viewAddEvent:
		return m.renderForm("New Event")
	case viewEditEvent:
		return m.renderForm("Edit Event")
	case viewDeleteConfirm:
		return m.renderDeleteConfirm()
	default:
		return m.renderCalendar()
	}
}

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
	weekdays := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
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
			Render("  No events")
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

	return b.String()
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
	timeStr := event.StartTime.Format("3:04 PM")
	if !event.AllDay {
		timeStr = fmt.Sprintf("%s - %s", event.StartTime.Format("3:04 PM"), event.EndTime.Format("3:04 PM"))
	} else {
		timeStr = "All day"
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

func (m *CalendarApp) renderForm(title string) string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(components.Primary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	labelStyle := lipgloss.NewStyle().Width(12).Foreground(components.Muted)
	focusedStyle := lipgloss.NewStyle().Foreground(components.Primary)

	// Title
	label := "Title:"
	if m.formFocusIdx == 0 {
		label = focusedStyle.Render(label)
	}
	b.WriteString(labelStyle.Render(label))
	b.WriteString(m.form.title.View())
	b.WriteString("\n")

	// Date
	label = "Date:"
	if m.formFocusIdx == 1 {
		label = focusedStyle.Render(label)
	}
	b.WriteString(labelStyle.Render(label))
	b.WriteString(m.form.date.View())
	b.WriteString("\n")

	// Start time
	label = "Start:"
	if m.formFocusIdx == 2 {
		label = focusedStyle.Render(label)
	}
	b.WriteString(labelStyle.Render(label))
	b.WriteString(m.form.start.View())
	b.WriteString("\n")

	// End time
	label = "End:"
	if m.formFocusIdx == 3 {
		label = focusedStyle.Render(label)
	}
	b.WriteString(labelStyle.Render(label))
	b.WriteString(m.form.end.View())
	b.WriteString("\n")

	// Location
	label = "Location:"
	if m.formFocusIdx == 4 {
		label = focusedStyle.Render(label)
	}
	b.WriteString(labelStyle.Render(label))
	b.WriteString(m.form.location.View())
	b.WriteString("\n")

	// Calendar selector
	label = "Calendar:"
	if m.formFocusIdx == 5 {
		label = focusedStyle.Render(label)
	}
	b.WriteString(labelStyle.Render(label))

	calName := "Default"
	if len(m.calendars) > 0 && m.form.calendar < len(m.calendars) {
		calName = m.calendars[m.form.calendar].Title
	}
	calStyle := lipgloss.NewStyle()
	if m.formFocusIdx == 5 {
		calStyle = calStyle.Background(components.Primary).Foreground(components.Text)
	}
	b.WriteString(calStyle.Render(fmt.Sprintf("◀ %s ▶", calName)))
	b.WriteString("\n\n")

	// Error
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(components.Danger)
		b.WriteString(errStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	// Help
	helpStyle := lipgloss.NewStyle().Foreground(components.Muted)
	b.WriteString(helpStyle.Render("Tab: next field  Ctrl+S: save  Esc: cancel"))

	return b.String()
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

	b.WriteString(titleStyle.Render("Delete Event?"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Are you sure you want to delete \"%s\"?\n\n", eventTitle))

	helpStyle := lipgloss.NewStyle().Foreground(components.Muted)
	b.WriteString(helpStyle.Render("y: yes  n: no"))

	return b.String()
}

func (m *CalendarApp) renderHelpBar() string {
	helpStyle := lipgloss.NewStyle().Foreground(components.Muted)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Secondary)

	help := []string{
		keyStyle.Render("←/→") + " day",
		keyStyle.Render("↑/↓") + " week",
		keyStyle.Render("h/l") + " month",
		keyStyle.Render("t") + " today",
		keyStyle.Render("a") + " add",
		keyStyle.Render("e") + " edit",
		keyStyle.Render("d") + " delete",
		keyStyle.Render("q") + " quit",
	}

	return helpStyle.Render(strings.Join(help, "  "))
}

// Key bindings
var keys = struct {
	Quit      key.Binding
	Left      key.Binding
	Right     key.Binding
	Up        key.Binding
	Down      key.Binding
	PrevMonth key.Binding
	NextMonth key.Binding
	Today     key.Binding
	Add       key.Binding
	Edit      key.Binding
	Delete    key.Binding
}{
	Quit:      key.NewBinding(key.WithKeys("q", "esc", "ctrl+c")),
	Left:      key.NewBinding(key.WithKeys("left", "h")),
	Right:     key.NewBinding(key.WithKeys("right", "l")),
	Up:        key.NewBinding(key.WithKeys("up", "k")),
	Down:      key.NewBinding(key.WithKeys("down", "j")),
	PrevMonth: key.NewBinding(key.WithKeys("H", "pageup")),
	NextMonth: key.NewBinding(key.WithKeys("L", "pagedown")),
	Today:     key.NewBinding(key.WithKeys("t")),
	Add:       key.NewBinding(key.WithKeys("a")),
	Edit:      key.NewBinding(key.WithKeys("e")),
	Delete:    key.NewBinding(key.WithKeys("d", "x")),
}
