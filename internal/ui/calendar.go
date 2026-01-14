package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
	viewEventDetail    // Event detail view
	viewNLPInput       // NLP quick-add: text input
	viewNLPParsing     // NLP quick-add: waiting for AI
	viewNLPEdit        // NLP quick-add: edit parsed event
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
	nlpInput       textarea.Model
	nlpParsed      *ai.ParsedEvent
	nlpCalendarIdx int
	nlpReminderIdx int
	nlpStartTime   time.Time
	nlpEndTime     time.Time

	// NLP edit fields (for editing parsed event)
	nlpEditTitle    textinput.Model
	nlpEditDate     components.DatePicker
	nlpEditStart    components.TimePicker
	nlpEditEnd      components.TimePicker
	nlpEditLocation textinput.Model
	nlpEditNotes    textarea.Model
	nlpEditFocus    int // 0=title, 1=date, 2=start, 3=end, 4=location, 5=notes

	// Interactive form fields (fallback when no AI CLI)
	formTitleInput    textinput.Model
	formDateInput     components.DatePicker
	formStartInput    components.TimePicker
	formEndInput      components.TimePicker
	formLocationInput textinput.Model
	formNotesInput    textarea.Model
	formCalendarIdx   int
	formReminderIdx   int
	formFocusField    int // 0=date, 1=start, 2=end, 3=location, 4=notes in datetime view

	// Delete confirmation
	deleteButtonIdx int // 0=Delete, 1=Cancel

	// Event detail view
	detailButtonIdx int // 0=Edit, 1=Delete, 2=Close
}

type eventForm struct {
	title    textinput.Model
	date     components.DatePicker
	start    components.TimePicker
	end      components.TimePicker
	location textinput.Model
	notes    textarea.Model
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
		// Resize NLP input textarea if in NLP input view
		if m.view == viewNLPInput {
			m.resizeNLPInput()
		}
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
		m.initNLPEdit()
		m.view = viewNLPEdit
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
		if m.view == viewNLPEdit {
			return m.handleNLPEditKeys(msg)
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
	case viewEventDetail:
		return m.handleEventDetailKeys(msg)
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
			return m, textarea.Blink
		} else {
			// Fallback to interactive form
			m.initInteractiveForm()
			m.view = viewFormTitle
		}
	case "enter":
		dayEvents := m.eventsForDate(m.selectedDate)
		if len(dayEvents) > 0 && m.selectedIdx < len(dayEvents) {
			m.detailButtonIdx = 0 // Default to Edit button
			m.view = viewEventDetail
		}
	case "e":
		dayEvents := m.eventsForDate(m.selectedDate)
		if len(dayEvents) > 0 && m.selectedIdx < len(dayEvents) {
			m.initEditForm(dayEvents[m.selectedIdx])
			m.view = viewEditEvent
		}
	case "d":
		dayEvents := m.eventsForDate(m.selectedDate)
		if len(dayEvents) > 0 && m.selectedIdx < len(dayEvents) {
			m.deleteButtonIdx = 0 // Default to "Delete" button
			m.view = viewDeleteConfirm
		}
	}
	return m, nil
}

func (m *CalendarApp) handleFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle date/time picker navigation (up/down/left/right when focused on date or time fields)
	if m.formFocusIdx >= 1 && m.formFocusIdx <= 3 {
		switch key {
		case "up", "down", "left", "right":
			switch m.formFocusIdx {
			case 1:
				m.form.date, _ = m.form.date.Update(msg)
			case 2:
				m.form.start, _ = m.form.start.Update(msg)
			case 3:
				m.form.end, _ = m.form.end.Update(msg)
			}
			return m, nil
		}
	}

	switch key {
	case "esc":
		m.view = viewCalendar
		return m, nil

	case "tab":
		m.formFocusIdx = (m.formFocusIdx + 1) % 8
		m.updateFormFocus()
		return m, nil

	case "shift+tab":
		m.formFocusIdx = (m.formFocusIdx + 7) % 8
		m.updateFormFocus()
		return m, nil

	case "enter":
		if m.formFocusIdx == 5 { // Calendar selector
			if len(m.calendars) > 0 {
				m.form.calendar = (m.form.calendar + 1) % len(m.calendars)
			}
			return m, nil
		}
		if m.formFocusIdx == 6 { // Save button
			return m, m.saveEvent()
		}
		if m.formFocusIdx == 7 { // Cancel button
			m.view = viewCalendar
			return m, nil
		}
		// Move to next field on enter
		m.formFocusIdx++
		m.updateFormFocus()
		return m, nil

	case "ctrl+s":
		return m, m.saveEvent()

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

	// Pass keystrokes to the focused text input (title, date, location only)
	var cmd tea.Cmd
	switch m.formFocusIdx {
	case 0:
		m.form.title, cmd = m.form.title.Update(msg)
	case 1:
		m.form.date, cmd = m.form.date.Update(msg)
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

func (m *CalendarApp) handleEventDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.view = viewCalendar
		return m, nil
	case "left", "h":
		if m.detailButtonIdx > 0 {
			m.detailButtonIdx--
		}
		return m, nil
	case "right", "l", "tab":
		if m.detailButtonIdx < 2 {
			m.detailButtonIdx++
		}
		return m, nil
	case "enter":
		dayEvents := m.eventsForDate(m.selectedDate)
		switch m.detailButtonIdx {
		case 0: // Edit
			if len(dayEvents) > 0 && m.selectedIdx < len(dayEvents) {
				m.initEditForm(dayEvents[m.selectedIdx])
				m.view = viewEditEvent
			}
		case 1: // Delete
			if len(dayEvents) > 0 && m.selectedIdx < len(dayEvents) {
				m.deleteButtonIdx = 0
				m.view = viewDeleteConfirm
			}
		case 2: // Close
			m.view = viewCalendar
		}
		return m, nil
	case "e":
		// Direct shortcut to edit
		dayEvents := m.eventsForDate(m.selectedDate)
		if len(dayEvents) > 0 && m.selectedIdx < len(dayEvents) {
			m.initEditForm(dayEvents[m.selectedIdx])
			m.view = viewEditEvent
		}
		return m, nil
	case "d":
		// Direct shortcut to delete
		dayEvents := m.eventsForDate(m.selectedDate)
		if len(dayEvents) > 0 && m.selectedIdx < len(dayEvents) {
			m.deleteButtonIdx = 0
			m.view = viewDeleteConfirm
		}
		return m, nil
	}
	return m, nil
}

func (m *CalendarApp) initEditForm(event calendar.Event) {
	m.form = eventForm{
		title:    textinput.New(),
		date:     components.NewDatePicker(),
		start:    components.NewTimePicker(),
		end:      components.NewTimePicker(),
		location: textinput.New(),
		editID:   event.ID,
	}

	m.form.title.SetValue(event.Title)
	m.form.title.Focus()

	m.form.date.SetDate(event.StartTime)
	m.form.start.SetTime24(event.StartTime.Format("15:04"))
	m.form.end.SetTime24(event.EndTime.Format("15:04"))
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
	m.nlpInput = textarea.New()
	m.nlpInput.Placeholder = "tomorrow 9am meeting with Jerry at the coffee shop\nto discuss project plans\n\nInclude: location, meeting URL, agenda, etc."
	m.nlpInput.CharLimit = 500
	m.nlpInput.ShowLineNumbers = false
	m.resizeNLPInput()
	m.nlpInput.Focus()
	m.nlpParsed = nil
	m.err = nil
}

// resizeNLPInput sets the NLP textarea dimensions based on window size
func (m *CalendarApp) resizeNLPInput() {
	// Set width based on window width, with min/max bounds
	width := min(max(m.width-10, 50), 80)
	m.nlpInput.SetWidth(width)
	// Height = 6 visible lines for multi-line NLP input
	m.nlpInput.SetHeight(6)
}

func (m *CalendarApp) handleNLPInputKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewCalendar
		return m, nil
	case "ctrl+enter":
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

// NLP Edit functions
func (m *CalendarApp) initNLPEdit() {
	m.nlpEditTitle = textinput.New()
	m.nlpEditTitle.SetValue(m.nlpParsed.Title)
	m.nlpEditTitle.Focus()
	m.nlpEditTitle.CharLimit = 100
	m.nlpEditTitle.Width = 40

	m.nlpEditDate = components.NewDatePicker()
	m.nlpEditDate.SetDate(m.nlpStartTime)

	m.nlpEditStart = components.NewTimePicker()
	m.nlpEditStart.SetTime24(m.nlpStartTime.Format("15:04"))

	m.nlpEditEnd = components.NewTimePicker()
	m.nlpEditEnd.SetTime24(m.nlpEndTime.Format("15:04"))

	m.nlpEditLocation = textinput.New()
	m.nlpEditLocation.SetValue(m.nlpParsed.Location)
	m.nlpEditLocation.Placeholder = "Location (optional)"
	m.nlpEditLocation.CharLimit = 100
	m.nlpEditLocation.Width = 40

	m.nlpEditNotes = textarea.New()
	m.nlpEditNotes.SetValue(m.nlpParsed.Notes)
	m.nlpEditNotes.Placeholder = "Notes: meeting URL, agenda, details..."
	m.nlpEditNotes.CharLimit = 1000
	m.nlpEditNotes.SetWidth(50)
	m.nlpEditNotes.SetHeight(4)
	m.nlpEditNotes.ShowLineNumbers = false

	m.nlpEditFocus = 0
}

func (m *CalendarApp) handleNLPEditKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle date/time picker navigation
	if m.nlpEditFocus >= 1 && m.nlpEditFocus <= 3 {
		switch key {
		case "up", "down", "left", "right":
			switch m.nlpEditFocus {
			case 1:
				m.nlpEditDate, _ = m.nlpEditDate.Update(msg)
			case 2:
				m.nlpEditStart, _ = m.nlpEditStart.Update(msg)
			case 3:
				m.nlpEditEnd, _ = m.nlpEditEnd.Update(msg)
			}
			return m, nil
		}
	}

	switch key {
	case "esc":
		m.view = viewCalendar
		return m, nil
	case "tab":
		m.nlpEditFocus = (m.nlpEditFocus + 1) % 6
		m.updateNLPEditFocus()
		return m, nil
	case "shift+tab":
		m.nlpEditFocus = (m.nlpEditFocus + 5) % 6
		m.updateNLPEditFocus()
		return m, nil
	case "enter":
		// Validate and apply edits
		if m.applyNLPEdits() {
			m.view = viewNLPCalendar
		}
		return m, nil
	}

	// Pass keystrokes to focused text input
	var cmd tea.Cmd
	switch m.nlpEditFocus {
	case 0:
		m.nlpEditTitle, cmd = m.nlpEditTitle.Update(msg)
	case 4:
		m.nlpEditLocation, cmd = m.nlpEditLocation.Update(msg)
	case 5:
		m.nlpEditNotes, cmd = m.nlpEditNotes.Update(msg)
	}
	return m, cmd
}

func (m *CalendarApp) updateNLPEditFocus() {
	m.nlpEditTitle.Blur()
	m.nlpEditDate.Blur()
	m.nlpEditStart.Blur()
	m.nlpEditEnd.Blur()
	m.nlpEditLocation.Blur()
	m.nlpEditNotes.Blur()

	switch m.nlpEditFocus {
	case 0:
		m.nlpEditTitle.Focus()
	case 1:
		m.nlpEditDate.Focus()
	case 2:
		m.nlpEditStart.Focus()
	case 3:
		m.nlpEditEnd.Focus()
	case 4:
		m.nlpEditLocation.Focus()
	case 5:
		m.nlpEditNotes.Focus()
	}
}

func (m *CalendarApp) applyNLPEdits() bool {
	// Update parsed event with edited values
	m.nlpParsed.Title = m.nlpEditTitle.Value()
	m.nlpParsed.Location = m.nlpEditLocation.Value()
	m.nlpParsed.Notes = m.nlpEditNotes.Value()

	// Update times
	date := m.nlpEditDate.Value()
	startTime, _ := time.Parse("15:04", m.nlpEditStart.Value24())
	endTime, _ := time.Parse("15:04", m.nlpEditEnd.Value24())

	// Validate end time is after start time
	if !endTime.After(startTime) {
		m.err = fmt.Errorf("end time must be after start time")
		return false
	}

	m.nlpStartTime = time.Date(date.Year(), date.Month(), date.Day(),
		startTime.Hour(), startTime.Minute(), 0, 0, time.Local)
	m.nlpEndTime = time.Date(date.Year(), date.Month(), date.Day(),
		endTime.Hour(), endTime.Minute(), 0, 0, time.Local)

	m.err = nil
	return true
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
			Notes:              m.nlpParsed.Notes,
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
		date := m.form.date.Value()

		startTime, err := time.Parse("15:04", m.form.start.Value24())
		if err != nil {
			return errMsg{fmt.Errorf("invalid start time: %v", err)}
		}

		endTime, err := time.Parse("15:04", m.form.end.Value24())
		if err != nil {
			return errMsg{fmt.Errorf("invalid end time: %v", err)}
		}

		// Validate end time is after start time
		if !endTime.After(startTime) {
			return errMsg{fmt.Errorf("end time must be after start time")}
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
			Notes:     m.form.notes.Value(),
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
	case viewEventDetail:
		return m.renderEventDetail()
	case viewNLPInput:
		return m.renderNLPInput()
	case viewNLPParsing:
		return m.renderNLPParsing()
	case viewNLPEdit:
		return m.renderNLPEdit()
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



// ============================================================================
// Interactive Form (fallback when no AI CLI available)
// ============================================================================

func (m *CalendarApp) initInteractiveForm() {
	m.formTitleInput = textinput.New()
	m.formTitleInput.Placeholder = "Meeting title"
	m.formTitleInput.Focus()
	m.formTitleInput.CharLimit = 100
	m.formTitleInput.Width = 50

	m.formDateInput = components.NewDatePicker()
	m.formDateInput.SetDate(m.selectedDate)

	m.formStartInput = components.NewTimePicker()
	m.formStartInput.SetTime24("09:00")

	m.formEndInput = components.NewTimePicker()
	m.formEndInput.SetTime24("10:00")

	m.formLocationInput = textinput.New()
	m.formLocationInput.Placeholder = "Location (optional)"
	m.formLocationInput.CharLimit = 100
	m.formLocationInput.Width = 40

	m.formNotesInput = textarea.New()
	m.formNotesInput.Placeholder = "Notes: meeting URL, agenda, details..."
	m.formNotesInput.CharLimit = 1000
	m.formNotesInput.SetWidth(50)
	m.formNotesInput.SetHeight(4)
	m.formNotesInput.ShowLineNumbers = false

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
			m.formNotesInput.Blur()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.formTitleInput, cmd = m.formTitleInput.Update(msg)
	return m, cmd
}

func (m *CalendarApp) handleFormDateTimeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle date/time picker navigation (up/down/left/right when focused on date or time fields)
	if m.formFocusField >= 0 && m.formFocusField <= 2 {
		switch key {
		case "up", "down", "left", "right":
			switch m.formFocusField {
			case 0:
				m.formDateInput, _ = m.formDateInput.Update(msg)
			case 1:
				m.formStartInput, _ = m.formStartInput.Update(msg)
			case 2:
				m.formEndInput, _ = m.formEndInput.Update(msg)
			}
			return m, nil
		}
	}

	switch key {
	case "esc":
		m.view = viewCalendar
		return m, nil
	case "tab":
		m.formFocusField = (m.formFocusField + 1) % 5
		m.updateFormDateTimeFocus()
		return m, nil
	case "shift+tab":
		m.formFocusField = (m.formFocusField + 4) % 5
		m.updateFormDateTimeFocus()
		return m, nil
	case "enter":
		// Validate and move to calendar selection
		if m.validateFormDateTime() {
			m.view = viewFormCalendar
		}
		return m, nil
	}

	// Pass keystrokes to text inputs
	var cmd tea.Cmd
	switch m.formFocusField {
	case 3:
		m.formLocationInput, cmd = m.formLocationInput.Update(msg)
	case 4:
		m.formNotesInput, cmd = m.formNotesInput.Update(msg)
	}
	return m, cmd
}

func (m *CalendarApp) updateFormDateTimeFocus() {
	m.formDateInput.Blur()
	m.formStartInput.Blur()
	m.formEndInput.Blur()
	m.formLocationInput.Blur()
	m.formNotesInput.Blur()

	switch m.formFocusField {
	case 0:
		m.formDateInput.Focus()
	case 1:
		m.formStartInput.Focus()
	case 2:
		m.formEndInput.Focus()
	case 3:
		m.formLocationInput.Focus()
	case 4:
		m.formNotesInput.Focus()
	}
}

func (m *CalendarApp) validateFormDateTime() bool {
	// components.DatePicker always has valid date values, no need to validate

	// Validate end time is after start time
	startTime, _ := time.Parse("15:04", m.formStartInput.Value24())
	endTime, _ := time.Parse("15:04", m.formEndInput.Value24())
	if !endTime.After(startTime) {
		m.err = fmt.Errorf("end time must be after start time")
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
	date := m.formDateInput.Value()
	startTime, _ := time.Parse("15:04", m.formStartInput.Value24())
	return time.Date(date.Year(), date.Month(), date.Day(),
		startTime.Hour(), startTime.Minute(), 0, 0, time.Local)
}

func (m *CalendarApp) getFormEndTime() time.Time {
	date := m.formDateInput.Value()
	endTime, _ := time.Parse("15:04", m.formEndInput.Value24())
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
			Notes:              m.formNotesInput.Value(),
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


