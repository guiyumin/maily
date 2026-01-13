package ui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/emersion/go-imap/v2"
	"maily/internal/auth"
	"maily/internal/calendar"
	"maily/internal/mail"
	"maily/internal/ui/components"
	"maily/internal/ui/utils"
)

// Panel focus
type panel int

const (
	emailPanel panel = iota
	eventPanel
)

// View state
type todayView int

const (
	todayDashboard todayView = iota
	todayEmailContent
	todayDeleteConfirm
	todayEditEvent
)

// AccountEmails holds emails for a single account
type AccountEmails struct {
	Email  string // account email address
	Emails []mail.Email
}

// TodayApp is the main today dashboard TUI model
type TodayApp struct {
	store        *auth.AccountStore
	calClient    calendar.Client
	imapClients  map[int]*mail.IMAPClient
	width        int
	height       int
	activePanel  panel
	view         todayView
	loading      bool
	loadingCount int // track how many accounts are still loading
	err          error

	// Email state (per account)
	accountEmails []AccountEmails
	emails        []mail.Email // flattened list for navigation
	emailCursor   int

	// Event state
	events      []calendar.Event
	eventCursor int

	// UI
	spinner  spinner.Model
	viewport viewport.Model

	// Edit event form
	editFormTitle    textinput.Model
	editFormDate     textinput.Model
	editFormStart    textinput.Model
	editFormEnd      textinput.Model
	editFormLocation textinput.Model
	editFormFocus    int
	editEventID      string
}

// Messages
type todayEmailsLoadedMsg struct {
	accountIdx int
	email      string // account email
	emails     []mail.Email
}

type todayEventsLoadedMsg struct {
	events []calendar.Event
}

type todayClientReadyMsg struct {
	accountIdx int
	imap       *mail.IMAPClient
}

type todayErrMsg struct {
	err error
}

type todayEmailDeletedMsg struct{}
type todayEventDeletedMsg struct{}
type todayEventUpdatedMsg struct{}

// NewTodayApp creates a new today dashboard TUI
func NewTodayApp(store *auth.AccountStore, calClient calendar.Client) *TodayApp {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = components.SpinnerStyle

	vp := viewport.New(80, 24)
	vp.Style = lipgloss.NewStyle().Padding(1, 2)

	return &TodayApp{
		store:         store,
		calClient:     calClient,
		imapClients:   make(map[int]*mail.IMAPClient),
		activePanel:   emailPanel,
		view:          todayDashboard,
		loading:       true,
		loadingCount:  len(store.Accounts),
		accountEmails: make([]AccountEmails, len(store.Accounts)),
		spinner:       s,
		viewport:      vp,
	}
}

func (m *TodayApp) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.spinner.Tick,
		m.loadTodayEvents(),
	}

	// Initialize clients for all accounts
	for i := range m.store.Accounts {
		cmds = append(cmds, m.initEmailClient(i))
	}

	return tea.Batch(cmds...)
}

func (m *TodayApp) initEmailClient(accountIdx int) tea.Cmd {
	return func() tea.Msg {
		if accountIdx >= len(m.store.Accounts) {
			return todayErrMsg{fmt.Errorf("invalid account index")}
		}

		account := m.store.Accounts[accountIdx]
		client, err := mail.NewIMAPClient(&account.Credentials)
		if err != nil {
			return todayErrMsg{err}
		}

		return todayClientReadyMsg{accountIdx: accountIdx, imap: client}
	}
}

func (m *TodayApp) loadTodayEmails(accountIdx int) tea.Cmd {
	return func() tea.Msg {
		client, ok := m.imapClients[accountIdx]
		if !ok || client == nil {
			return todayErrMsg{fmt.Errorf("email client not ready for account %d", accountIdx)}
		}

		account := m.store.Accounts[accountIdx]

		// Load today's emails (received today)
		emails, err := client.FetchMessages("INBOX", 50)
		if err != nil {
			return todayErrMsg{err}
		}

		// Filter to today's emails only
		today := time.Now()
		todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

		var todayEmails []mail.Email
		for _, email := range emails {
			if email.Date.After(todayStart) || email.Date.Equal(todayStart) {
				todayEmails = append(todayEmails, email)
			}
		}

		return todayEmailsLoadedMsg{
			accountIdx: accountIdx,
			email:      account.Credentials.Email,
			emails:     todayEmails,
		}
	}
}

func (m *TodayApp) loadTodayEvents() tea.Cmd {
	return func() tea.Msg {
		today := time.Now()
		start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
		end := start.Add(24 * time.Hour)

		events, err := m.calClient.ListEvents(start, end)
		if err != nil {
			return todayErrMsg{err}
		}

		return todayEventsLoadedMsg{events}
	}
}

func (m *TodayApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 8
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case todayClientReadyMsg:
		m.imapClients[msg.accountIdx] = msg.imap
		return m, m.loadTodayEmails(msg.accountIdx)

	case todayEmailsLoadedMsg:
		// Store emails for this account
		m.accountEmails[msg.accountIdx] = AccountEmails{
			Email:  msg.email,
			Emails: msg.emails,
		}
		// Rebuild flattened list
		m.rebuildEmailList()
		m.loadingCount--
		if m.loadingCount <= 0 {
			m.loading = false
		}
		return m, nil

	case todayEventsLoadedMsg:
		m.events = msg.events
		return m, nil

	case todayErrMsg:
		m.err = msg.err
		m.loading = false
		return m, nil

	case todayEmailDeletedMsg:
		// Already handled locally in handleDeleteConfirm
		return m, nil

	case todayEventDeletedMsg:
		// Remove event from local list and reload
		if m.eventCursor < len(m.events) {
			m.events = append(m.events[:m.eventCursor], m.events[m.eventCursor+1:]...)
			if m.eventCursor >= len(m.events) && m.eventCursor > 0 {
				m.eventCursor--
			}
		}
		m.view = todayDashboard
		return m, m.loadTodayEvents()

	case todayEventUpdatedMsg:
		m.view = todayDashboard
		return m, m.loadTodayEvents()

	case tea.KeyMsg:
		// Route to appropriate handler based on view
		switch m.view {
		case todayDeleteConfirm:
			return m.handleDeleteConfirm(msg)
		case todayEditEvent:
			return m.handleEditEvent(msg)
		default:
			return m.handleKeyPress(msg)
		}
	}

	return m, nil
}

func (m *TodayApp) rebuildEmailList() {
	// Flatten all account emails into a single list for navigation
	m.emails = nil
	for _, acc := range m.accountEmails {
		m.emails = append(m.emails, acc.Emails...)
	}
}

func (m *TodayApp) findAccountForEmail(emailIdx int) int {
	// Find which account the email at emailIdx belongs to
	idx := 0
	for i, acc := range m.accountEmails {
		if emailIdx < idx+len(acc.Emails) {
			return i
		}
		idx += len(acc.Emails)
	}
	return 0
}

func (m *TodayApp) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle email content view
	if m.view == todayEmailContent {
		switch msg.String() {
		case "q", "ctrl+c":
			for _, client := range m.imapClients {
				if client != nil {
					client.Close()
				}
			}
			return m, tea.Quit
		case "esc":
			m.view = todayDashboard
			return m, nil
		case "up":
			m.viewport.ScrollUp(3)
			return m, nil
		case "down":
			m.viewport.ScrollDown(3)
			return m, nil
		case "d":
			// Delete current email
			m.view = todayDeleteConfirm
			return m, nil
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	// Dashboard view
	switch msg.String() {
	case "q", "ctrl+c":
		for _, client := range m.imapClients {
			if client != nil {
				client.Close()
			}
		}
		return m, tea.Quit

	case "tab":
		// Switch panels
		if m.activePanel == emailPanel {
			m.activePanel = eventPanel
		} else {
			m.activePanel = emailPanel
		}

	case "up":
		if m.activePanel == emailPanel {
			if m.emailCursor > 0 {
				m.emailCursor--
			}
		} else {
			if m.eventCursor > 0 {
				m.eventCursor--
			}
		}

	case "down":
		if m.activePanel == emailPanel {
			if m.emailCursor < len(m.emails)-1 {
				m.emailCursor++
			}
		} else {
			if m.eventCursor < len(m.events)-1 {
				m.eventCursor++
			}
		}

	case "enter":
		// Open selected email
		if m.activePanel == emailPanel && len(m.emails) > 0 && m.emailCursor < len(m.emails) {
			email := m.emails[m.emailCursor]
			m.view = todayEmailContent
			m.viewport.SetContent(m.renderEmailContent(email))
			m.viewport.GotoTop()

			// Mark as read - find the right client and update local state
			if email.Unread {
				// Update local state immediately for responsive UI
				m.markEmailAsRead(m.emailCursor)

				accountIdx := m.findAccountForEmail(m.emailCursor)
				if client, ok := m.imapClients[accountIdx]; ok && client != nil {
					uid := email.UID
					go func() {
						client.MarkAsRead(uid)
					}()
				}
			}
		}

	case "r":
		// Refresh
		m.loading = true
		m.loadingCount = len(m.store.Accounts)
		cmds := []tea.Cmd{
			m.spinner.Tick,
			m.loadTodayEvents(),
		}
		for i := range m.store.Accounts {
			cmds = append(cmds, m.loadTodayEmails(i))
		}
		return m, tea.Batch(cmds...)

	case "d":
		// Delete selected item
		if m.activePanel == emailPanel && len(m.emails) > 0 {
			m.view = todayDeleteConfirm
		} else if m.activePanel == eventPanel && len(m.events) > 0 {
			m.view = todayDeleteConfirm
		}

	case "e":
		// Edit event (only for events panel)
		if m.activePanel == eventPanel && len(m.events) > 0 && m.eventCursor < len(m.events) {
			m.initEditEventForm(m.events[m.eventCursor])
			m.view = todayEditEvent
		}
	}

	return m, nil
}

func (m *TodayApp) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		if m.activePanel == emailPanel && len(m.emails) > 0 && m.emailCursor < len(m.emails) {
			// Delete email
			email := m.emails[m.emailCursor]
			uid := email.UID
			accountIdx := m.findAccountForEmail(m.emailCursor)
			if client, ok := m.imapClients[accountIdx]; ok && client != nil {
				go func() {
					client.DeleteMessage(uid)
				}()
			}
			// Remove from cache by UID
			m.removeEmailByUID(uid)
			m.view = todayDashboard
		} else if m.activePanel == eventPanel && len(m.events) > 0 && m.eventCursor < len(m.events) {
			// Delete event
			event := m.events[m.eventCursor]
			return m, m.deleteEvent(event.ID)
		}
		return m, nil

	case "n", "N", "esc":
		m.view = todayDashboard
		return m, nil
	}
	return m, nil
}

func (m *TodayApp) handleEditEvent(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = todayDashboard
		return m, nil

	case "tab":
		m.editFormFocus = (m.editFormFocus + 1) % 5
		m.updateEditFormFocus()
		return m, nil

	case "shift+tab":
		m.editFormFocus = (m.editFormFocus + 4) % 5
		m.updateEditFormFocus()
		return m, nil

	case "enter":
		// Save on last field or move to next
		if m.editFormFocus < 4 {
			m.editFormFocus++
			m.updateEditFormFocus()
			return m, nil
		}
		return m, m.saveEditedEvent()

	case "ctrl+s":
		if runtime.GOOS != "darwin" {
			return m, m.saveEditedEvent()
		}
	case "cmd+s":
		if runtime.GOOS == "darwin" {
			return m, m.saveEditedEvent()
		}
	}

	// Pass keystrokes to focused field
	var cmd tea.Cmd
	switch m.editFormFocus {
	case 0:
		m.editFormTitle, cmd = m.editFormTitle.Update(msg)
	case 1:
		m.editFormDate, cmd = m.editFormDate.Update(msg)
	case 2:
		m.editFormStart, cmd = m.editFormStart.Update(msg)
	case 3:
		m.editFormEnd, cmd = m.editFormEnd.Update(msg)
	case 4:
		m.editFormLocation, cmd = m.editFormLocation.Update(msg)
	}
	return m, cmd
}

func (m *TodayApp) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.loading {
		return m.renderLoading()
	}

	if m.err != nil {
		return m.renderError()
	}

	switch m.view {
	case todayEmailContent:
		return m.renderEmailView()
	case todayDeleteConfirm:
		return m.renderDeleteConfirm()
	case todayEditEvent:
		return m.renderEditEventForm()
	default:
		return m.renderDashboard()
	}
}

func (m *TodayApp) renderLoading() string {
	content := lipgloss.NewStyle().
		Foreground(components.Text).
		Render(m.spinner.View() + " Loading today's dashboard...")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *TodayApp) renderError() string {
	content := lipgloss.NewStyle().
		Foreground(components.Danger).
		Render(fmt.Sprintf("Error: %v", m.err))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *TodayApp) renderDashboard() string {
	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(components.Primary).
		Padding(1, 2)
	title := titleStyle.Render("maily")

	// Calculate panel widths (60% emails, 40% events)
	contentHeight := m.height - 6 // title + footer
	emailWidth := m.width * 6 / 10
	eventWidth := m.width - emailWidth - 3

	// Render panels
	emailPanelView := m.renderEmailPanel(emailWidth, contentHeight)
	eventPanelView := m.renderEventPanel(eventWidth, contentHeight)

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, emailPanelView, eventPanelView)

	// Help bar
	helpBar := m.renderHelpBar()

	return lipgloss.JoinVertical(lipgloss.Left, title, panels, helpBar)
}

func (m *TodayApp) renderEmailPanel(width, height int) string {
	var b strings.Builder

	// Panel title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Muted)
	if m.activePanel == emailPanel {
		titleStyle = titleStyle.Foreground(components.Text)
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Today's Emails (%d)", len(m.emails))))
	b.WriteString("\n")

	// Separator line
	separatorStyle := lipgloss.NewStyle().Foreground(components.Muted)
	b.WriteString(separatorStyle.Render(strings.Repeat("─", width-4)))
	b.WriteString("\n")

	// Email list grouped by account
	if len(m.emails) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(components.Muted).Italic(true)
		b.WriteString(emptyStyle.Render("  No emails today"))
	} else {
		// Track global index for cursor
		globalIdx := 0

		for _, acc := range m.accountEmails {
			if len(acc.Emails) == 0 {
				continue
			}

			// Account header (only show if multiple accounts)
			if len(m.accountEmails) > 1 {
				accountStyle := lipgloss.NewStyle().Foreground(components.Secondary).Bold(true)
				b.WriteString(accountStyle.Render(acc.Email))
				b.WriteString("\n")
			}

			// Emails for this account
			for _, email := range acc.Emails {
				indent := ""
				if len(m.accountEmails) > 1 {
					indent = "  " // indent if multiple accounts
				}
				line := m.renderCompactEmailLine(email, globalIdx == m.emailCursor, width-4-len(indent))
				b.WriteString(indent + line)
				b.WriteString("\n")
				globalIdx++
			}
		}
	}

	panelStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(0, 1).
		BorderRight(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(components.Muted)

	return panelStyle.Render(b.String())
}

func (m *TodayApp) renderCompactEmailLine(email mail.Email, isCursor bool, maxWidth int) string {
	// Format: [●] Subject (truncated)
	var prefix string
	if email.Unread {
		prefix = lipgloss.NewStyle().Foreground(components.Primary).Render("● ")
	} else {
		prefix = lipgloss.NewStyle().Foreground(components.Muted).Render("○ ")
	}

	// Cursor indicator
	if isCursor && m.activePanel == emailPanel {
		prefix = lipgloss.NewStyle().Foreground(components.Primary).Render("▸ ")
	}

	subject := email.Subject
	if subject == "" {
		subject = "(no subject)"
	}

	// Truncate subject
	availWidth := maxWidth - 4 // prefix + padding
	if len(subject) > availWidth {
		subject = subject[:availWidth-3] + "..."
	}

	style := lipgloss.NewStyle().Foreground(components.Text)
	if isCursor && m.activePanel == emailPanel {
		style = style.Bold(true).Background(components.Primary)
	} else if email.Unread {
		style = style.Bold(true)
	}

	return prefix + style.Render(subject)
}

func (m *TodayApp) renderEventPanel(width, height int) string {
	var b strings.Builder

	// Panel title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Muted)
	if m.activePanel == eventPanel {
		titleStyle = titleStyle.Foreground(components.Text)
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Events (%d)", len(m.events))))
	b.WriteString("\n")

	// Separator line
	separatorStyle := lipgloss.NewStyle().Foreground(components.Muted)
	b.WriteString(separatorStyle.Render(strings.Repeat("─", width-4)))
	b.WriteString("\n")

	// Event list (vertical timeline)
	if len(m.events) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(components.Muted).Italic(true)
		b.WriteString(emptyStyle.Render("  No events today"))
	} else {
		for i, event := range m.events {
			line := m.renderEventLine(event, i == m.eventCursor, width-4)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	panelStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(0, 1)

	return panelStyle.Render(b.String())
}

func (m *TodayApp) renderEventLine(event calendar.Event, isCursor bool, maxWidth int) string {
	var b strings.Builder

	// Time on first line
	timeStr := event.StartTime.Format("3:04pm")
	if event.AllDay {
		timeStr = "All day"
	}

	timeStyle := lipgloss.NewStyle().Foreground(components.Muted)
	titleStyle := lipgloss.NewStyle().Foreground(components.Text)

	if isCursor && m.activePanel == eventPanel {
		timeStyle = timeStyle.Foreground(components.Primary).Bold(true)
		titleStyle = titleStyle.Bold(true).Background(components.Primary)
	}

	// Time line
	b.WriteString(timeStyle.Render(timeStr))
	b.WriteString("\n")

	// Title line (indented)
	prefix := " "
	if isCursor && m.activePanel == eventPanel {
		prefix = lipgloss.NewStyle().Foreground(components.Primary).Render("▸")
	}

	// Truncate title
	title := event.Title
	availWidth := maxWidth - 3
	if len(title) > availWidth {
		title = title[:availWidth-3] + "..."
	}

	b.WriteString(prefix)
	b.WriteString(titleStyle.Render(title))

	return b.String()
}

func (m *TodayApp) renderHelpBar() string {
	helpStyle := lipgloss.NewStyle().Foreground(components.Muted).Padding(1, 2)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Secondary)

	items := []string{
		keyStyle.Render("↑↓") + " navigate",
		keyStyle.Render("tab") + " switch",
		keyStyle.Render("enter") + " open",
		keyStyle.Render("d") + " delete",
	}

	// Show edit only for events panel
	if m.activePanel == eventPanel {
		items = append(items, keyStyle.Render("e")+" edit")
	}

	items = append(items,
		keyStyle.Render("r")+" refresh",
		keyStyle.Render("q")+" quit",
	)

	return helpStyle.Render(strings.Join(items, "  "))
}

func (m *TodayApp) renderEmailView() string {
	if m.emailCursor >= len(m.emails) {
		return ""
	}
	email := m.emails[m.emailCursor]

	// Header
	headerStyle := lipgloss.NewStyle().Padding(0, 2)
	fromStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Secondary)
	labelStyle := lipgloss.NewStyle().Foreground(components.Muted)
	subjectStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Text)

	header := headerStyle.Render(
		labelStyle.Render("From: ") + fromStyle.Render(email.From) + "\n" +
			labelStyle.Render("To: ") + email.To + "\n" +
			labelStyle.Render("Subject: ") + subjectStyle.Render(email.Subject) + "\n" +
			labelStyle.Render("Date: ") + email.Date.Format("Mon, Jan 2, 2006 3:04 PM"),
	)

	// Separator
	separator := lipgloss.NewStyle().
		Foreground(components.Muted).
		Render(strings.Repeat("─", m.width-4))

	// Help
	helpStyle := lipgloss.NewStyle().Foreground(components.Muted).Padding(0, 2)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(components.Secondary)
	help := helpStyle.Render(keyStyle.Render("esc") + " back  " + keyStyle.Render("↑↓") + " scroll  " + keyStyle.Render("d") + " delete  " + keyStyle.Render("q") + " quit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		separator,
		m.viewport.View(),
		help,
	)
}

func (m *TodayApp) renderEmailContent(email mail.Email) string {
	body := email.BodyHTML
	if body == "" {
		body = email.Snippet
	}
	if body == "" {
		return "(no content)"
	}
	// Render HTML with glamour
	width := m.viewport.Width - 4
	if width < 40 {
		width = 40
	}
	return components.RenderHTMLBody(body, width)
}

// Helper functions for delete/edit

func (m *TodayApp) removeEmailByUID(uid imap.UID) {
	// Search through all accounts and remove email with matching UID
	for i := range m.accountEmails {
		for j, email := range m.accountEmails[i].Emails {
			if email.UID == uid {
				m.accountEmails[i].Emails = append(
					m.accountEmails[i].Emails[:j],
					m.accountEmails[i].Emails[j+1:]...,
				)
				// Rebuild flattened list
				m.rebuildEmailList()
				// Adjust cursor if needed
				if m.emailCursor >= len(m.emails) && m.emailCursor > 0 {
					m.emailCursor--
				}
				return
			}
		}
	}
}

func (m *TodayApp) markEmailAsRead(emailIdx int) {
	// Update in the flattened list
	if emailIdx >= 0 && emailIdx < len(m.emails) {
		m.emails[emailIdx].Unread = false
	}

	// Also update in the per-account list
	idx := 0
	for i := range m.accountEmails {
		for j := range m.accountEmails[i].Emails {
			if idx == emailIdx {
				m.accountEmails[i].Emails[j].Unread = false
				return
			}
			idx++
		}
	}
}

func (m *TodayApp) deleteEvent(eventID string) tea.Cmd {
	return func() tea.Msg {
		err := m.calClient.DeleteEvent(eventID)
		if err != nil {
			return todayErrMsg{err}
		}
		return todayEventDeletedMsg{}
	}
}

func (m *TodayApp) initEditEventForm(event calendar.Event) {
	m.editFormTitle = textinput.New()
	m.editFormTitle.SetValue(event.Title)
	m.editFormTitle.Focus()

	m.editFormDate = textinput.New()
	m.editFormDate.Placeholder = "YYYY-MM-DD"
	m.editFormDate.SetValue(event.StartTime.Format("2006-01-02"))

	m.editFormStart = textinput.New()
	m.editFormStart.Placeholder = "HH:MM"
	m.editFormStart.SetValue(event.StartTime.Format("15:04"))

	m.editFormEnd = textinput.New()
	m.editFormEnd.Placeholder = "HH:MM"
	m.editFormEnd.SetValue(event.EndTime.Format("15:04"))

	m.editFormLocation = textinput.New()
	m.editFormLocation.Placeholder = "Location (optional)"
	m.editFormLocation.SetValue(event.Location)

	m.editFormFocus = 0
	m.editEventID = event.ID
}

func (m *TodayApp) updateEditFormFocus() {
	m.editFormTitle.Blur()
	m.editFormDate.Blur()
	m.editFormStart.Blur()
	m.editFormEnd.Blur()
	m.editFormLocation.Blur()

	switch m.editFormFocus {
	case 0:
		m.editFormTitle.Focus()
	case 1:
		m.editFormDate.Focus()
	case 2:
		m.editFormStart.Focus()
	case 3:
		m.editFormEnd.Focus()
	case 4:
		m.editFormLocation.Focus()
	}
}

func (m *TodayApp) saveEditedEvent() tea.Cmd {
	return func() tea.Msg {
		date, err := time.Parse("2006-01-02", m.editFormDate.Value())
		if err != nil {
			return todayErrMsg{fmt.Errorf("invalid date: %v", err)}
		}

		startTime, err := time.Parse("15:04", m.editFormStart.Value())
		if err != nil {
			return todayErrMsg{fmt.Errorf("invalid start time: %v", err)}
		}

		endTime, err := time.Parse("15:04", m.editFormEnd.Value())
		if err != nil {
			return todayErrMsg{fmt.Errorf("invalid end time: %v", err)}
		}

		start := time.Date(date.Year(), date.Month(), date.Day(),
			startTime.Hour(), startTime.Minute(), 0, 0, time.Local)
		end := time.Date(date.Year(), date.Month(), date.Day(),
			endTime.Hour(), endTime.Minute(), 0, 0, time.Local)

		// Delete old event first (EventKit doesn't have update)
		if m.editEventID != "" {
			_ = m.calClient.DeleteEvent(m.editEventID)
		}

		event := calendar.Event{
			Title:     m.editFormTitle.Value(),
			StartTime: start,
			EndTime:   end,
			Location:  m.editFormLocation.Value(),
		}

		_, err = m.calClient.CreateEvent(event)
		if err != nil {
			return todayErrMsg{err}
		}

		return todayEventUpdatedMsg{}
	}
}

func (m *TodayApp) renderDeleteConfirm() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(components.Danger)

	b.WriteString(titleStyle.Render("Delete?"))
	b.WriteString("\n\n")

	if m.activePanel == emailPanel && m.emailCursor < len(m.emails) {
		email := m.emails[m.emailCursor]
		b.WriteString(fmt.Sprintf("Delete email: \"%s\"?\n\n", utils.TruncateStr(email.Subject, 40)))
	} else if m.activePanel == eventPanel && m.eventCursor < len(m.events) {
		event := m.events[m.eventCursor]
		b.WriteString(fmt.Sprintf("Delete event: \"%s\"?\n\n", utils.TruncateStr(event.Title, 40)))
	}

	helpStyle := lipgloss.NewStyle().Foreground(components.Muted)
	b.WriteString(helpStyle.Render("y: yes  n: no"))

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(components.Danger).
		Padding(1, 3)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialogStyle.Render(b.String()))
}

func (m *TodayApp) renderEditEventForm() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(components.Primary)

	b.WriteString(titleStyle.Render("Edit Event"))
	b.WriteString("\n\n")

	labelStyle := lipgloss.NewStyle().Width(12).Foreground(components.Muted)
	focusedStyle := lipgloss.NewStyle().Foreground(components.Primary)

	// Title
	label := "Title:"
	if m.editFormFocus == 0 {
		label = focusedStyle.Render(label)
	}
	b.WriteString(labelStyle.Render(label))
	b.WriteString(m.editFormTitle.View())
	b.WriteString("\n")

	// Date
	label = "Date:"
	if m.editFormFocus == 1 {
		label = focusedStyle.Render(label)
	}
	b.WriteString(labelStyle.Render(label))
	b.WriteString(m.editFormDate.View())
	b.WriteString("\n")

	// Start time
	label = "Start:"
	if m.editFormFocus == 2 {
		label = focusedStyle.Render(label)
	}
	b.WriteString(labelStyle.Render(label))
	b.WriteString(m.editFormStart.View())
	b.WriteString("\n")

	// End time
	label = "End:"
	if m.editFormFocus == 3 {
		label = focusedStyle.Render(label)
	}
	b.WriteString(labelStyle.Render(label))
	b.WriteString(m.editFormEnd.View())
	b.WriteString("\n")

	// Location
	label = "Location:"
	if m.editFormFocus == 4 {
		label = focusedStyle.Render(label)
	}
	b.WriteString(labelStyle.Render(label))
	b.WriteString(m.editFormLocation.View())
	b.WriteString("\n\n")

	// Help
	helpStyle := lipgloss.NewStyle().Foreground(components.Muted)
	saveKey := "Ctrl+S"
	if runtime.GOOS == "darwin" {
		saveKey = "⌘+S"
	}
	b.WriteString(helpStyle.Render(fmt.Sprintf("Tab: next field  %s: save  Esc: cancel", saveKey)))

	formStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(components.Primary).
		Padding(1, 3)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, formStyle.Render(b.String()))
}

