package ui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"maily/internal/ai"
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
	viewNLPInput       // NLP quick-add: text input
	viewNLPParsing     // NLP quick-add: waiting for AI
	viewNLPCalendar    // NLP quick-add: select calendar
	viewNLPReminder    // NLP quick-add: select reminder
	viewNLPConfirm     // NLP quick-add: confirm
	viewFormTitle      // Interactive form: title input
	viewFormDateTime   // Interactive form: date/time input
	viewFormCalendar   // Interactive form: select calendar
	viewFormReminder   // Interactive form: select reminder
	viewFormConfirm    // Interactive form: confirm
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
	pendingKey   string // for two-key combos like w+↓
	err          error

	// Form fields for add/edit
	form         eventForm
	formFocusIdx int

	// NLP quick-add fields
	nlpInput       textinput.Model
	nlpParsed      *ai.ParsedEvent
	nlpCalendarIdx int
	nlpReminderIdx int
	nlpStartTime   time.Time
	nlpEndTime     time.Time

	// Interactive form fields (fallback when no AI CLI)
	formTitleInput    textinput.Model
	formDateInput     textinput.Model
	formStartInput    textinput.Model
	formEndInput      textinput.Model
	formLocationInput textinput.Model
	formCalendarIdx   int
	formReminderIdx   int
	formFocusField    int // 0=date, 1=start, 2=end, 3=location in datetime view

	// Delete confirmation
	deleteButtonIdx int // 0=Delete, 1=Cancel
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

type nlpParsedMsg struct {
	parsed    *ai.ParsedEvent
	startTime time.Time
	endTime   time.Time
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
		m.view = viewCalendar
		return m, nil

	case nlpParsedMsg:
		m.nlpParsed = msg.parsed
		m.nlpStartTime = msg.startTime
		m.nlpEndTime = msg.endTime
		m.nlpCalendarIdx = 0
		m.nlpReminderIdx = 0
		m.view = viewNLPCalendar
		return m, nil

	case tea.KeyMsg:
		// Handle form input first if in form view
		if m.view == viewAddEvent || m.view == viewEditEvent {
			return m.handleFormInput(msg)
		}
		// Handle NLP views
		if m.view == viewNLPInput {
			return m.handleNLPInputKeys(msg)
		}
		if m.view == viewNLPCalendar || m.view == viewNLPReminder || m.view == viewNLPConfirm {
			return m.handleNLPSelectKeys(msg)
		}
		// Handle interactive form views
		if m.view == viewFormTitle {
			return m.handleFormTitleKeys(msg)
		}
		if m.view == viewFormDateTime {
			return m.handleFormDateTimeKeys(msg)
		}
		if m.view == viewFormCalendar || m.view == viewFormReminder || m.view == viewFormConfirm {
			return m.handleFormSelectKeys(msg)
		}
		return m.handleKeyPress(msg)
	}

	return m, nil
}

func (m *CalendarApp) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.view {
	case viewCalendar:
		return m.handleCalendarKeys(msg)
	case viewDeleteConfirm:
		return m.handleDeleteKeys(msg)
	}
	return m, nil
}

func (m *CalendarApp) handleCalendarKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// Month/Year mode: m/y sets mode, up/down navigates, esc exits
	if m.pendingKey != "" {
		switch k {
		case "up", "down":
			dir := 1
			if k == "up" {
				dir = -1
			}
			switch m.pendingKey {
			case "m":
				m.selectedDate = m.selectedDate.AddDate(0, dir, 0)
			case "y":
				m.selectedDate = m.selectedDate.AddDate(dir, 0, 0)
			}
			m.selectedIdx = 0
			return m, m.loadEvents()
		case "esc", "q":
			m.pendingKey = ""
			return m, nil
		case "m", "y":
			m.pendingKey = k
			return m, nil
		default:
			return m, nil // Ignore other keys in mode
		}
	}

	switch k {
	case "m", "y":
		m.pendingKey = k
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	case "left":
		m.selectedDate = m.selectedDate.AddDate(0, 0, -1)
		m.selectedIdx = 0
	case "right":
		m.selectedDate = m.selectedDate.AddDate(0, 0, 1)
		m.selectedIdx = 0
	case "up":
		m.selectedDate = m.selectedDate.AddDate(0, 0, -7)
		m.selectedIdx = 0
		return m, m.loadEvents()
	case "down":
		m.selectedDate = m.selectedDate.AddDate(0, 0, 7)
		m.selectedIdx = 0
		return m, m.loadEvents()
	case "tab":
		dayEvents := m.eventsForDate(m.selectedDate)
		if len(dayEvents) > 0 {
			m.selectedIdx = (m.selectedIdx + 1) % len(dayEvents)
		}
	case "shift+tab":
		dayEvents := m.eventsForDate(m.selectedDate)
		if len(dayEvents) > 0 {
			m.selectedIdx = (m.selectedIdx + len(dayEvents) - 1) % len(dayEvents)
		}
	case "t":
		m.selectedDate = time.Now()
		m.selectedIdx = 0
		return m, m.loadEvents()
	case "n":
		// Check if AI CLI is available
		aiClient := ai.NewClient()
		if aiClient.Available() {
			// NLP quick-add (AI-powered)
			m.initNLPInput()
			m.view = viewNLPInput
		} else {
			// Fallback to interactive form
			m.initInteractiveForm()
			m.view = viewFormTitle
		}
	case "e":
		dayEvents := m.eventsForDate(m.selectedDate)
		if len(dayEvents) > 0 && m.selectedIdx < len(dayEvents) {
			m.initEditForm(dayEvents[m.selectedIdx])
			m.view = viewEditEvent
		}
	case "d", "x", "backspace":
		dayEvents := m.eventsForDate(m.selectedDate)
		if len(dayEvents) > 0 && m.selectedIdx < len(dayEvents) {
			m.deleteButtonIdx = 0 // Default to "Delete" button
			m.view = viewDeleteConfirm
		}
	}
	return m, nil
}

func (m *CalendarApp) handleFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewCalendar
		return m, nil

	case "tab":
		m.formFocusIdx = (m.formFocusIdx + 1) % 6
		m.updateFormFocus()
		return m, nil

	case "shift+tab":
		m.formFocusIdx = (m.formFocusIdx + 5) % 6
		m.updateFormFocus()
		return m, nil

	case "enter":
		if m.formFocusIdx == 5 { // Calendar selector
			if len(m.calendars) > 0 {
				m.form.calendar = (m.form.calendar + 1) % len(m.calendars)
			}
			return m, nil
		}
		// Move to next field on enter, save on last field
		if m.formFocusIdx < 4 {
			m.formFocusIdx++
			m.updateFormFocus()
			return m, nil
		}
		return m, m.saveEvent()

	case "cmd+s", "ctrl+s":
		// cmd+s for macOS, ctrl+s for Windows/Linux
		if msg.String() == "cmd+s" && runtime.GOOS == "darwin" {
			return m, m.saveEvent()
		}
		if msg.String() == "ctrl+s" && runtime.GOOS != "darwin" {
			return m, m.saveEvent()
		}
		return m, nil

	case "left":
		if m.formFocusIdx == 5 && len(m.calendars) > 0 {
			m.form.calendar = (m.form.calendar + len(m.calendars) - 1) % len(m.calendars)
			return m, nil
		}

	case "right":
		if m.formFocusIdx == 5 && len(m.calendars) > 0 {
			m.form.calendar = (m.form.calendar + 1) % len(m.calendars)
			return m, nil
		}
	}

	// Pass keystrokes to the focused text input
	var cmd tea.Cmd
	switch m.formFocusIdx {
	case 0:
		m.form.title, cmd = m.form.title.Update(msg)
	case 1:
		m.form.date, cmd = m.form.date.Update(msg)
	case 2:
		m.form.start, cmd = m.form.start.Update(msg)
	case 3:
		m.form.end, cmd = m.form.end.Update(msg)
	case 4:
		m.form.location, cmd = m.form.location.Update(msg)
	}

	return m, cmd
}

func (m *CalendarApp) handleDeleteKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		if m.deleteButtonIdx > 0 {
			m.deleteButtonIdx--
		}
	case "right", "l", "tab":
		if m.deleteButtonIdx < 1 {
			m.deleteButtonIdx++
		}
	case "enter":
		if m.deleteButtonIdx == 0 {
			// Delete
			dayEvents := m.eventsForDate(m.selectedDate)
			if m.selectedIdx < len(dayEvents) {
				return m, m.deleteEvent(dayEvents[m.selectedIdx].ID)
			}
		}
		// Cancel (or if delete index out of range)
		m.view = viewCalendar
		return m, nil
	case "esc", "q":
		m.view = viewCalendar
		return m, nil
	}

	return m, nil
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

// NLP Quick-Add functions
func (m *CalendarApp) initNLPInput() {
	m.nlpInput = textinput.New()
	m.nlpInput.Placeholder = "tomorrow 9am meeting with Jerry"
	m.nlpInput.Focus()
	m.nlpInput.CharLimit = 200
	m.nlpInput.Width = 50
	m.nlpParsed = nil
	m.err = nil
}

func (m *CalendarApp) handleNLPInputKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewCalendar
		return m, nil
	case "enter":
		if m.nlpInput.Value() != "" {
			m.view = viewNLPParsing
			return m, m.parseNLPInput()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.nlpInput, cmd = m.nlpInput.Update(msg)
	return m, cmd
}

func (m *CalendarApp) handleNLPSelectKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewCalendar
		return m, nil
	case "up", "k":
		switch m.view {
		case viewNLPCalendar:
			if m.nlpCalendarIdx > 0 {
				m.nlpCalendarIdx--
			}
		case viewNLPReminder:
			if m.nlpReminderIdx > 0 {
				m.nlpReminderIdx--
			}
		}
	case "down", "j":
		switch m.view {
		case viewNLPCalendar:
			if m.nlpCalendarIdx < len(m.calendars)-1 {
				m.nlpCalendarIdx++
			}
		case viewNLPReminder:
			if m.nlpReminderIdx < 5 { // 6 options: 0, 5, 10, 15, 30, 60
				m.nlpReminderIdx++
			}
		}
	case "enter":
		switch m.view {
		case viewNLPCalendar:
			m.view = viewNLPReminder
		case viewNLPReminder:
			m.view = viewNLPConfirm
		case viewNLPConfirm:
			return m, m.createNLPEvent()
		}
	}
	return m, nil
}

func (m *CalendarApp) parseNLPInput() tea.Cmd {
	return func() tea.Msg {
		aiClient := ai.NewClient()
		if !aiClient.Available() {
			return errMsg{fmt.Errorf("no AI CLI found (install claude, codex, gemini, or ollama)")}
		}

		prompt := ai.ParseCalendarEventPrompt(m.nlpInput.Value(), time.Now())
		response, err := aiClient.Call(prompt)
		if err != nil {
			return errMsg{err}
		}

		parsed, err := ai.ParseEventResponse(response)
		if err != nil {
			return errMsg{err}
		}

		startTime, err := parsed.GetStartTime()
		if err != nil {
			return errMsg{fmt.Errorf("invalid start time: %v", err)}
		}

		endTime, err := parsed.GetEndTime()
		if err != nil {
			return errMsg{fmt.Errorf("invalid end time: %v", err)}
		}

		return nlpParsedMsg{
			parsed:    parsed,
			startTime: startTime,
			endTime:   endTime,
		}
	}
}

func (m *CalendarApp) getNLPReminderMinutes() int {
	reminderOptions := []int{0, 5, 10, 15, 30, 60}
	if m.nlpReminderIdx < len(reminderOptions) {
		return reminderOptions[m.nlpReminderIdx]
	}
	return 0
}

func (m *CalendarApp) createNLPEvent() tea.Cmd {
	return func() tea.Msg {
		var calendarID string
		if len(m.calendars) > 0 && m.nlpCalendarIdx < len(m.calendars) {
			calendarID = m.calendars[m.nlpCalendarIdx].ID
		}

		event := calendar.Event{
			Title:              m.nlpParsed.Title,
			StartTime:          m.nlpStartTime,
			EndTime:            m.nlpEndTime,
			Location:           m.nlpParsed.Location,
			Calendar:           calendarID,
			AlarmMinutesBefore: m.getNLPReminderMinutes(),
		}

		id, err := m.client.CreateEvent(event)
		if err != nil {
			return errMsg{err}
		}

		return eventCreatedMsg{id}
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
	case viewNLPInput:
		return m.renderNLPInput()
	case viewNLPParsing:
		return m.renderNLPParsing()
	case viewNLPCalendar:
		return m.renderNLPCalendar()
	case viewNLPReminder:
		return m.renderNLPReminder()
	case viewNLPConfirm:
		return m.renderNLPConfirm()
	case viewFormTitle:
		return m.renderFormTitle()
	case viewFormDateTime:
		return m.renderFormDateTime()
	case viewFormCalendar:
		return m.renderFormCalendar()
	case viewFormReminder:
		return m.renderFormReminder()
	case viewFormConfirm:
		return m.renderFormConfirm()
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
		timeStr = "All day"
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

func (m *CalendarApp) renderForm(title string) string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(components.Primary)

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
	saveKey := "Ctrl+S"
	if runtime.GOOS == "darwin" {
		saveKey = "⌘ + S"
	}
	b.WriteString(helpStyle.Render(fmt.Sprintf("Tab: next field  %s: save  Esc: cancel", saveKey)))

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

	b.WriteString(titleStyle.Render("Delete Event?"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Are you sure you want to delete \"%s\"?\n\n", eventTitle))

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
		deleteBtn = selectedBtn.Render("Delete")
		cancelBtn = unselectedBtn.Render("Cancel")
	} else {
		deleteBtn = unselectedBtn.Render("Delete")
		cancelBtn = selectedBtn.Background(components.Muted).Foreground(lipgloss.Color("#FFFFFF")).Render("Cancel")
	}

	b.WriteString(deleteBtn + "  " + cancelBtn)
	b.WriteString("\n\n")

	hintStyle := lipgloss.NewStyle().Foreground(components.Muted)
	b.WriteString(hintStyle.Render("←/→ select • enter confirm • esc cancel"))

	// Wrap with padding
	dialogStyle := lipgloss.NewStyle().Padding(1, 2)
	return dialogStyle.Render(b.String())
}

func (m *CalendarApp) renderHelpBar() string {
	helpStyle := lipgloss.NewStyle().Foreground(components.Muted)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Secondary)
	modeStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Primary)

	// Show mode indicator when in navigation mode
	if m.pendingKey != "" {
		modeName := map[string]string{"m": "MONTH", "y": "YEAR"}[m.pendingKey]
		return modeStyle.Render("["+modeName+" MODE]") + "  " +
			helpStyle.Render(keyStyle.Render("↑↓")+" navigate  "+keyStyle.Render("esc")+" exit mode")
	}

	// Row 1: Navigation
	row1 := []string{
		keyStyle.Render("←→") + " day",
		keyStyle.Render("↑↓") + " week",
		keyStyle.Render("tab") + " event",
		keyStyle.Render("m") + " month",
		keyStyle.Render("y") + " year",
		keyStyle.Render("t") + " today",
	}

	// Row 2: Actions
	row2 := []string{
		keyStyle.Render("n") + " new event",
		keyStyle.Render("e") + " edit",
		keyStyle.Render("x") + " delete",
		keyStyle.Render("q") + " quit",
	}

	return helpStyle.Render(strings.Join(row1, "  ")) + "\n" +
		helpStyle.Render(strings.Join(row2, "  "))
}

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
	b.WriteString(fmt.Sprintf("  │  Title:    %-35s│\n", truncateStr(m.nlpParsed.Title, 35)))
	b.WriteString(fmt.Sprintf("  │  Date:     %-35s│\n", m.nlpStartTime.Format("Monday, Jan 2, 2006")))
	b.WriteString(fmt.Sprintf("  │  Time:     %-35s│\n", fmt.Sprintf("%s - %s", m.nlpStartTime.Format("3:04 PM"), m.nlpEndTime.Format("3:04 PM"))))
	if m.nlpParsed.Location != "" {
		b.WriteString(fmt.Sprintf("  │  Location: %-35s│\n", truncateStr(m.nlpParsed.Location, 35)))
	}
	calName := "Default"
	if len(m.calendars) > 0 && m.nlpCalendarIdx < len(m.calendars) {
		calName = m.calendars[m.nlpCalendarIdx].Title
	}
	b.WriteString(fmt.Sprintf("  │  Calendar: %-35s│\n", truncateStr(calName, 35)))
	reminderStr := "None"
	if mins := m.getNLPReminderMinutes(); mins > 0 {
		if mins == 60 {
			reminderStr = "1 hour before"
		} else {
			reminderStr = fmt.Sprintf("%d minutes before", mins)
		}
	}
	b.WriteString(fmt.Sprintf("  │  Reminder: %-35s│\n", reminderStr))
	b.WriteString("  └────────────────────────────────────────────────┘\n")

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("enter create • esc cancel"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *CalendarApp) renderNLPEventBox() string {
	var b strings.Builder

	b.WriteString("  ┌─ Parsed Event ─────────────────────────────────┐\n")
	b.WriteString(fmt.Sprintf("  │  Title:    %-37s│\n", truncateStr(m.nlpParsed.Title, 37)))
	b.WriteString(fmt.Sprintf("  │  Date:     %-37s│\n", m.nlpStartTime.Format("Monday, Jan 2, 2006")))
	b.WriteString(fmt.Sprintf("  │  Time:     %-37s│\n", fmt.Sprintf("%s - %s", m.nlpStartTime.Format("3:04 PM"), m.nlpEndTime.Format("3:04 PM"))))
	if m.nlpParsed.Location != "" {
		b.WriteString(fmt.Sprintf("  │  Location: %-37s│\n", truncateStr(m.nlpParsed.Location, 37)))
	}
	b.WriteString("  └────────────────────────────────────────────────┘")

	return b.String()
}

// ============================================================================
// Interactive Form (fallback when no AI CLI available)
// ============================================================================

func (m *CalendarApp) initInteractiveForm() {
	m.formTitleInput = textinput.New()
	m.formTitleInput.Placeholder = "Meeting title"
	m.formTitleInput.Focus()
	m.formTitleInput.CharLimit = 100
	m.formTitleInput.Width = 50

	m.formDateInput = textinput.New()
	m.formDateInput.Placeholder = "YYYY-MM-DD"
	m.formDateInput.SetValue(m.selectedDate.Format("2006-01-02"))
	m.formDateInput.CharLimit = 10
	m.formDateInput.Width = 15

	m.formStartInput = textinput.New()
	m.formStartInput.Placeholder = "HH:MM"
	m.formStartInput.SetValue("09:00")
	m.formStartInput.CharLimit = 5
	m.formStartInput.Width = 10

	m.formEndInput = textinput.New()
	m.formEndInput.Placeholder = "HH:MM"
	m.formEndInput.SetValue("10:00")
	m.formEndInput.CharLimit = 5
	m.formEndInput.Width = 10

	m.formLocationInput = textinput.New()
	m.formLocationInput.Placeholder = "Location (optional)"
	m.formLocationInput.CharLimit = 100
	m.formLocationInput.Width = 40

	m.formCalendarIdx = 0
	m.formReminderIdx = 0
	m.formFocusField = 0
	m.err = nil
}

func (m *CalendarApp) handleFormTitleKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewCalendar
		return m, nil
	case "enter":
		if m.formTitleInput.Value() != "" {
			m.view = viewFormDateTime
			m.formFocusField = 0
			m.formDateInput.Focus()
			m.formStartInput.Blur()
			m.formEndInput.Blur()
			m.formLocationInput.Blur()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.formTitleInput, cmd = m.formTitleInput.Update(msg)
	return m, cmd
}

func (m *CalendarApp) handleFormDateTimeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewCalendar
		return m, nil
	case "tab":
		m.formFocusField = (m.formFocusField + 1) % 4
		m.updateFormDateTimeFocus()
		return m, nil
	case "shift+tab":
		m.formFocusField = (m.formFocusField + 3) % 4
		m.updateFormDateTimeFocus()
		return m, nil
	case "enter":
		// Validate and move to calendar selection
		if m.validateFormDateTime() {
			m.view = viewFormCalendar
		}
		return m, nil
	}

	// Pass keystrokes to the focused input
	var cmd tea.Cmd
	switch m.formFocusField {
	case 0:
		m.formDateInput, cmd = m.formDateInput.Update(msg)
	case 1:
		m.formStartInput, cmd = m.formStartInput.Update(msg)
	case 2:
		m.formEndInput, cmd = m.formEndInput.Update(msg)
	case 3:
		m.formLocationInput, cmd = m.formLocationInput.Update(msg)
	}
	return m, cmd
}

func (m *CalendarApp) updateFormDateTimeFocus() {
	m.formDateInput.Blur()
	m.formStartInput.Blur()
	m.formEndInput.Blur()
	m.formLocationInput.Blur()

	switch m.formFocusField {
	case 0:
		m.formDateInput.Focus()
	case 1:
		m.formStartInput.Focus()
	case 2:
		m.formEndInput.Focus()
	case 3:
		m.formLocationInput.Focus()
	}
}

func (m *CalendarApp) validateFormDateTime() bool {
	_, err := time.Parse("2006-01-02", m.formDateInput.Value())
	if err != nil {
		m.err = fmt.Errorf("invalid date format (use YYYY-MM-DD)")
		return false
	}

	_, err = time.Parse("15:04", m.formStartInput.Value())
	if err != nil {
		m.err = fmt.Errorf("invalid start time (use HH:MM)")
		return false
	}

	_, err = time.Parse("15:04", m.formEndInput.Value())
	if err != nil {
		m.err = fmt.Errorf("invalid end time (use HH:MM)")
		return false
	}

	m.err = nil
	return true
}

func (m *CalendarApp) handleFormSelectKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewCalendar
		return m, nil
	case "up", "k":
		switch m.view {
		case viewFormCalendar:
			if m.formCalendarIdx > 0 {
				m.formCalendarIdx--
			}
		case viewFormReminder:
			if m.formReminderIdx > 0 {
				m.formReminderIdx--
			}
		}
	case "down", "j":
		switch m.view {
		case viewFormCalendar:
			if m.formCalendarIdx < len(m.calendars)-1 {
				m.formCalendarIdx++
			}
		case viewFormReminder:
			if m.formReminderIdx < 5 {
				m.formReminderIdx++
			}
		}
	case "enter":
		switch m.view {
		case viewFormCalendar:
			m.view = viewFormReminder
		case viewFormReminder:
			m.view = viewFormConfirm
		case viewFormConfirm:
			return m, m.createFormEvent()
		}
	}
	return m, nil
}

func (m *CalendarApp) getFormReminderMinutes() int {
	reminderOptions := []int{0, 5, 10, 15, 30, 60}
	if m.formReminderIdx < len(reminderOptions) {
		return reminderOptions[m.formReminderIdx]
	}
	return 0
}

func (m *CalendarApp) getFormStartTime() time.Time {
	date, _ := time.Parse("2006-01-02", m.formDateInput.Value())
	startTime, _ := time.Parse("15:04", m.formStartInput.Value())
	return time.Date(date.Year(), date.Month(), date.Day(),
		startTime.Hour(), startTime.Minute(), 0, 0, time.Local)
}

func (m *CalendarApp) getFormEndTime() time.Time {
	date, _ := time.Parse("2006-01-02", m.formDateInput.Value())
	endTime, _ := time.Parse("15:04", m.formEndInput.Value())
	return time.Date(date.Year(), date.Month(), date.Day(),
		endTime.Hour(), endTime.Minute(), 0, 0, time.Local)
}

func (m *CalendarApp) createFormEvent() tea.Cmd {
	return func() tea.Msg {
		var calendarID string
		if len(m.calendars) > 0 && m.formCalendarIdx < len(m.calendars) {
			calendarID = m.calendars[m.formCalendarIdx].ID
		}

		event := calendar.Event{
			Title:              m.formTitleInput.Value(),
			StartTime:          m.getFormStartTime(),
			EndTime:            m.getFormEndTime(),
			Location:           m.formLocationInput.Value(),
			Calendar:           calendarID,
			AlarmMinutesBefore: m.getFormReminderMinutes(),
		}

		id, err := m.client.CreateEvent(event)
		if err != nil {
			return errMsg{err}
		}

		return eventCreatedMsg{id}
	}
}

// Interactive form render functions

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
	b.WriteString(fmt.Sprintf("  │  Title: %-41s│\n", truncateStr(m.formTitleInput.Value(), 41)))
	b.WriteString("  └────────────────────────────────────────────────────┘\n\n")

	b.WriteString("  When is it?\n\n")

	// Date
	label := "Date:"
	if m.formFocusField == 0 {
		b.WriteString("    " + focusedLabel.Render(label) + m.formDateInput.View() + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render(label) + m.formDateInput.View() + "\n")
	}

	// Start time
	label = "Start:"
	if m.formFocusField == 1 {
		b.WriteString("    " + focusedLabel.Render(label) + m.formStartInput.View() + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render(label) + m.formStartInput.View() + "\n")
	}

	// End time
	label = "End:"
	if m.formFocusField == 2 {
		b.WriteString("    " + focusedLabel.Render(label) + m.formEndInput.View() + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render(label) + m.formEndInput.View() + "\n")
	}

	// Location
	label = "Location:"
	if m.formFocusField == 3 {
		b.WriteString("    " + focusedLabel.Render(label) + m.formLocationInput.View() + "\n")
	} else {
		b.WriteString("    " + labelStyle.Render(label) + m.formLocationInput.View() + "\n")
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
	b.WriteString(fmt.Sprintf("  │  Title:    %-35s│\n", truncateStr(m.formTitleInput.Value(), 35)))
	b.WriteString(fmt.Sprintf("  │  Date:     %-35s│\n", startTime.Format("Monday, Jan 2, 2006")))
	b.WriteString(fmt.Sprintf("  │  Time:     %-35s│\n", fmt.Sprintf("%s - %s", startTime.Format("3:04 PM"), endTime.Format("3:04 PM"))))
	if m.formLocationInput.Value() != "" {
		b.WriteString(fmt.Sprintf("  │  Location: %-35s│\n", truncateStr(m.formLocationInput.Value(), 35)))
	}
	calName := "Default"
	if len(m.calendars) > 0 && m.formCalendarIdx < len(m.calendars) {
		calName = m.calendars[m.formCalendarIdx].Title
	}
	b.WriteString(fmt.Sprintf("  │  Calendar: %-35s│\n", truncateStr(calName, 35)))
	reminderStr := "None"
	if mins := m.getFormReminderMinutes(); mins > 0 {
		if mins == 60 {
			reminderStr = "1 hour before"
		} else {
			reminderStr = fmt.Sprintf("%d minutes before", mins)
		}
	}
	b.WriteString(fmt.Sprintf("  │  Reminder: %-35s│\n", reminderStr))
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
	b.WriteString(fmt.Sprintf("  │  Title:    %-39s│\n", truncateStr(m.formTitleInput.Value(), 39)))
	b.WriteString(fmt.Sprintf("  │  Date:     %-39s│\n", startTime.Format("Monday, Jan 2, 2006")))
	b.WriteString(fmt.Sprintf("  │  Time:     %-39s│\n", fmt.Sprintf("%s - %s", startTime.Format("3:04 PM"), endTime.Format("3:04 PM"))))
	if m.formLocationInput.Value() != "" {
		b.WriteString(fmt.Sprintf("  │  Location: %-39s│\n", truncateStr(m.formLocationInput.Value(), 39)))
	}
	b.WriteString("  └────────────────────────────────────────────────────┘")

	return b.String()
}

