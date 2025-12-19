package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/emersion/go-imap/v2"

	"maily/internal/auth"
	"maily/internal/gmail"
	"maily/internal/ui/components"
)

type searchState int

const (
	searchStateLoading searchState = iota
	searchStateReady
	searchStateConfirm
	searchStateExecuting
	searchStateDone
	searchStateError
)

type actionType int

const (
	actionNone actionType = iota
	actionDelete
	actionArchive
	actionMarkRead
)

type searchView int

const (
	searchListView searchView = iota
	searchReadView
)

type SearchApp struct {
	account           *auth.Account
	query             string
	imap              *gmail.IMAPClient
	emails            []gmail.Email
	selected          map[int]bool
	cursor            int
	state             searchState
	view              searchView
	action            actionType
	spinner           spinner.Model
	viewport          viewport.Model
	width             int
	height            int
	err               error
	message           string
	scrollCount       int
	confirmDeleteSingle bool
}

type searchResultsMsg struct {
	emails []gmail.Email
}

type searchErrorMsg struct {
	err error
}

type actionCompleteMsg struct {
	count int
}

func NewSearchApp(account *auth.Account, query string) SearchApp {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = components.SpinnerStyle

	vp := viewport.New(80, 24)
	vp.Style = lipgloss.NewStyle().Padding(1, 4, 3, 4)

	return SearchApp{
		account:  account,
		query:    query,
		selected: make(map[int]bool),
		state:    searchStateLoading,
		view:     searchListView,
		spinner:  s,
		viewport: vp,
	}
}

func (a SearchApp) Init() tea.Cmd {
	return tea.Batch(
		a.spinner.Tick,
		a.connect(),
	)
}

func (a SearchApp) connect() tea.Cmd {
	return func() tea.Msg {
		client, err := gmail.NewIMAPClient(&a.account.Credentials)
		if err != nil {
			return searchErrorMsg{err: err}
		}

		emails, err := client.SearchMessages("INBOX", a.query)
		if err != nil {
			client.Close()
			return searchErrorMsg{err: err}
		}

		return searchResultsMsg{emails: emails}
	}
}

func (a *SearchApp) executeAction() tea.Cmd {
	return func() tea.Msg {
		var uids []imap.UID
		for i, email := range a.emails {
			if a.selected[i] {
				uids = append(uids, email.UID)
			}
		}

		var err error
		switch a.action {
		case actionDelete:
			err = a.imap.DeleteMessages(uids)
		case actionArchive:
			err = a.imap.ArchiveMessages(uids)
		case actionMarkRead:
			err = a.imap.MarkMessagesAsRead(uids)
		}

		if err != nil {
			return searchErrorMsg{err: err}
		}

		return actionCompleteMsg{count: len(uids)}
	}
}

func (a SearchApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch a.state {
		case searchStateReady:
			return a.handleReadyKeys(msg)
		case searchStateConfirm:
			return a.handleConfirmKeys(msg)
		case searchStateDone:
			if msg.String() == "q" || msg.String() == "enter" || msg.String() == "esc" {
				return a, tea.Quit
			}
		case searchStateError:
			if msg.String() == "q" || msg.String() == "enter" || msg.String() == "esc" {
				return a, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.viewport.Width = msg.Width - 8
		a.viewport.Height = msg.Height - 8

	case tea.MouseMsg:
		if a.state == searchStateReady {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				if a.view == searchReadView {
					a.viewport.ScrollUp(3)
				} else {
					// Only process every 3rd scroll event
					a.scrollCount++
					if a.scrollCount >= 3 {
						if a.cursor > 0 {
							a.cursor--
						}
						a.scrollCount = 0
					}
				}
				return a, nil
			case tea.MouseButtonWheelDown:
				if a.view == searchReadView {
					a.viewport.ScrollDown(3)
				} else {
					// Only process every 3rd scroll event
					a.scrollCount++
					if a.scrollCount >= 3 {
						if a.cursor < len(a.emails)-1 {
							a.cursor++
						}
						a.scrollCount = 0
					}
				}
				return a, nil
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd

	case searchResultsMsg:
		a.emails = msg.emails
		a.state = searchStateReady
		if len(a.emails) == 0 {
			a.message = "No emails found matching your query."
			a.state = searchStateDone
		}
		// Store the IMAP client for later actions
		client, _ := gmail.NewIMAPClient(&a.account.Credentials)
		if client != nil {
			client.SelectMailbox("INBOX")
		}
		a.imap = client

	case searchErrorMsg:
		a.state = searchStateError
		a.err = msg.err

	case actionCompleteMsg:
		actionName := ""
		switch a.action {
		case actionDelete:
			actionName = "deleted"
			// Remove deleted emails from the list
			var remaining []gmail.Email
			for i, email := range a.emails {
				if !a.selected[i] {
					remaining = append(remaining, email)
				}
			}
			a.emails = remaining
		case actionArchive:
			actionName = "archived"
			// Remove archived emails from the list
			var remaining []gmail.Email
			for i, email := range a.emails {
				if !a.selected[i] {
					remaining = append(remaining, email)
				}
			}
			a.emails = remaining
		case actionMarkRead:
			actionName = "marked as read"
			// Update unread status in the list
			for i := range a.emails {
				if a.selected[i] {
					a.emails[i].Unread = false
				}
			}
		}

		// Clear selections and reset state
		a.selected = make(map[int]bool)
		a.action = actionNone

		// Adjust cursor if needed
		if a.cursor >= len(a.emails) && a.cursor > 0 {
			a.cursor = len(a.emails) - 1
		}

		// Go back to ready state (list view)
		a.state = searchStateReady
		a.message = fmt.Sprintf("Successfully %s %d email(s).", actionName, msg.count)

		// If no emails left, show done state
		if len(a.emails) == 0 {
			a.message = fmt.Sprintf("Successfully %s %d email(s). No more results.", actionName, msg.count)
			a.state = searchStateDone
		}
	}

	return a, nil
}

func (a SearchApp) handleReadyKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle read view keys
	if a.view == searchReadView {
		return a.handleReadViewKeys(msg)
	}

	// Clear message on any interaction
	a.message = ""

	// Handle list view keys
	switch msg.String() {
	case "q":
		if a.imap != nil {
			a.imap.Close()
		}
		return a, tea.Quit

	case "esc":
		if a.imap != nil {
			a.imap.Close()
		}
		return a, tea.Quit

	case "enter":
		// Open selected email
		if len(a.emails) > 0 && a.cursor < len(a.emails) {
			email := a.emails[a.cursor]
			a.view = searchReadView
			a.viewport.SetContent(a.renderEmailContent(email))
			a.viewport.GotoTop()

			// Mark as read in background
			if email.Unread && a.imap != nil {
				go func() {
					a.imap.MarkAsRead(email.UID)
				}()
			}
		}

	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
		}

	case "down", "j":
		if a.cursor < len(a.emails)-1 {
			a.cursor++
		}

	case " ": // Space to toggle selection
		a.selected[a.cursor] = !a.selected[a.cursor]
		if a.cursor < len(a.emails)-1 {
			a.cursor++
		}

	case "a": // Select all
		allSelected := len(a.selected) == len(a.emails)
		if allSelected {
			// Deselect all
			a.selected = make(map[int]bool)
		} else {
			// Select all
			for i := range a.emails {
				a.selected[i] = true
			}
		}

	case "d": // Delete
		if a.selectedCount() > 0 {
			a.action = actionDelete
			a.state = searchStateConfirm
		}

	case "e": // Archive (Gmail: move to All Mail)
		if a.selectedCount() > 0 {
			a.action = actionArchive
			a.state = searchStateConfirm
		}

	case "r": // Mark as read
		if a.selectedCount() > 0 {
			a.action = actionMarkRead
			a.state = searchStateConfirm
		}
	}

	return a, nil
}

func (a SearchApp) handleReadViewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		if a.imap != nil {
			a.imap.Close()
		}
		return a, tea.Quit

	case "esc":
		// Go back to list view
		a.view = searchListView
		a.confirmDeleteSingle = false

	case "d":
		// Delete current email
		if !a.confirmDeleteSingle {
			a.confirmDeleteSingle = true
		}

	case "y":
		if a.confirmDeleteSingle {
			// Delete the current email
			if a.cursor < len(a.emails) {
				email := a.emails[a.cursor]
				// Remove from list
				a.emails = append(a.emails[:a.cursor], a.emails[a.cursor+1:]...)
				// Adjust cursor if needed
				if a.cursor >= len(a.emails) && a.cursor > 0 {
					a.cursor--
				}
				// Delete in background
				if a.imap != nil {
					go func() {
						a.imap.DeleteMessage(email.UID)
					}()
				}
				// Go back to list view
				a.view = searchListView
				a.confirmDeleteSingle = false
			}
		}

	case "n":
		if a.confirmDeleteSingle {
			a.confirmDeleteSingle = false
		}

	case "up", "k":
		a.viewport.ScrollUp(1)

	case "down", "j":
		a.viewport.ScrollDown(1)

	case "pgup":
		a.viewport.ScrollUp(a.viewport.Height)

	case "pgdown":
		a.viewport.ScrollDown(a.viewport.Height)
	}

	return a, nil
}

func (a SearchApp) renderEmailContent(email gmail.Email) string {
	body := email.Body
	if body == "" {
		body = email.Snippet
	}
	return body
}

func (a SearchApp) handleConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		a.state = searchStateExecuting
		return a, tea.Batch(a.spinner.Tick, a.executeAction())

	case "n", "N", "esc":
		a.state = searchStateReady
		a.action = actionNone
	}

	return a, nil
}

func (a SearchApp) selectedCount() int {
	count := 0
	for _, selected := range a.selected {
		if selected {
			count++
		}
	}
	return count
}

func (a SearchApp) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var content string

	switch a.state {
	case searchStateLoading:
		content = lipgloss.Place(
			a.width,
			a.height-4,
			lipgloss.Center,
			lipgloss.Center,
			fmt.Sprintf("%s Searching...", a.spinner.View()),
		)

	case searchStateReady:
		switch a.view {
		case searchListView:
			content = a.renderResults()
		case searchReadView:
			content = a.renderReadView()
		}

	case searchStateConfirm:
		content = a.renderConfirmDialog()

	case searchStateExecuting:
		content = lipgloss.Place(
			a.width,
			a.height-4,
			lipgloss.Center,
			lipgloss.Center,
			fmt.Sprintf("%s Executing...", a.spinner.View()),
		)

	case searchStateDone:
		content = lipgloss.Place(
			a.width,
			a.height-4,
			lipgloss.Center,
			lipgloss.Center,
			components.SuccessStyle.Render(a.message+"\n\nPress Enter or q to exit."),
		)

	case searchStateError:
		content = lipgloss.Place(
			a.width,
			a.height-4,
			lipgloss.Center,
			lipgloss.Center,
			components.ErrorStyle.Render(fmt.Sprintf("Error: %v\n\nPress Enter or q to exit.", a.err)),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		a.renderHeader(),
		content,
		a.renderStatusBar(),
	)
}

func (a SearchApp) renderHeader() string {
	title := components.TitleStyle.Render(" MAILY SEARCH ")

	queryInfo := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Render(fmt.Sprintf("Query: %s", a.query))

	return components.HeaderStyle.Width(a.width).Render(title + "  " + queryInfo)
}

func (a SearchApp) renderResults() string {
	var b strings.Builder

	visibleHeight := a.height - 8
	if visibleHeight < 1 {
		visibleHeight = 10
	}

	start := 0
	if a.cursor >= visibleHeight {
		start = a.cursor - visibleHeight + 1
	}

	end := start + visibleHeight
	if end > len(a.emails) {
		end = len(a.emails)
	}

	for i := start; i < end; i++ {
		email := a.emails[i]
		line := a.renderEmailLine(email, i == a.cursor, a.selected[i])
		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (a SearchApp) renderReadView() string {
	if a.cursor >= len(a.emails) {
		return ""
	}

	email := a.emails[a.cursor]

	// Email header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Padding(0, 2)

	fromLine := headerStyle.Render(fmt.Sprintf("From: %s", email.From))
	toLine := headerStyle.Render(fmt.Sprintf("To: %s", email.To))
	subjectLine := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F9FAFB")).
		Padding(0, 2).
		Render(fmt.Sprintf("Subject: %s", email.Subject))
	dateLine := headerStyle.Render(fmt.Sprintf("Date: %s", email.Date.Format("Mon, 02 Jan 2006 15:04:05")))

	header := lipgloss.JoinVertical(lipgloss.Left,
		fromLine,
		toLine,
		subjectLine,
		dateLine,
		"",
	)

	// Confirmation overlay if deleting
	if a.confirmDeleteSingle {
		confirmDialog := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#EF4444")).
			Padding(1, 3).
			Align(lipgloss.Center).
			Render(
				lipgloss.JoinVertical(lipgloss.Center,
					lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444")).Render("Delete this email?"),
					"",
					lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render("press y to confirm, n to cancel"),
				),
			)

		return lipgloss.Place(
			a.width,
			a.height-4,
			lipgloss.Center,
			lipgloss.Center,
			confirmDialog,
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		a.viewport.View(),
	)
}

func (a SearchApp) renderEmailLine(email gmail.Email, cursor bool, selected bool) string {
	maxWidth := a.width - 14
	if maxWidth < 40 {
		maxWidth = 80
	}

	from := truncateStr(extractNameFromEmail(email.From), 20)
	subject := truncateStr(email.Subject, maxWidth-35)
	date := formatEmailDate(email.Date)

	// Selection indicator
	var checkbox string
	if selected {
		checkbox = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render(" [✓] ")
	} else {
		checkbox = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" [ ] ")
	}

	// Unread indicator
	var status string
	if email.Unread {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Render("● ")
	} else {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("○ ")
	}

	line := fmt.Sprintf("%-20s │ %-*s │ %s",
		from,
		maxWidth-35,
		subject,
		date,
	)

	lineStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Width(a.width - 14)

	if cursor {
		lineStyle = lineStyle.
			Bold(true).
			Background(lipgloss.Color("#7C3AED"))
	} else if selected {
		lineStyle = lineStyle.
			Foreground(lipgloss.Color("#10B981"))
	} else if email.Unread {
		lineStyle = lineStyle.Bold(true)
	}

	return checkbox + status + lineStyle.Render(line)
}

func (a SearchApp) renderConfirmDialog() string {
	actionName := ""
	actionColor := lipgloss.Color("#EF4444")

	switch a.action {
	case actionDelete:
		actionName = "Delete"
		actionColor = lipgloss.Color("#EF4444")
	case actionArchive:
		actionName = "Archive"
		actionColor = lipgloss.Color("#F59E0B")
	case actionMarkRead:
		actionName = "Mark as read"
		actionColor = lipgloss.Color("#3B82F6")
	}

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(actionColor).
		Padding(1, 3).
		Align(lipgloss.Center)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(actionColor).
		Render(fmt.Sprintf("%s %d email(s)?", actionName, a.selectedCount()))

	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Render("press y to confirm, n to cancel")

	return lipgloss.Place(
		a.width,
		a.height-4,
		lipgloss.Center,
		lipgloss.Center,
		dialogStyle.Render(
			lipgloss.JoinVertical(
				lipgloss.Center,
				title,
				"",
				hint,
			),
		),
	)
}

func (a SearchApp) renderStatusBar() string {
	var help string

	switch a.state {
	case searchStateReady:
		if a.view == searchReadView {
			// Read view help
			help = components.HelpKeyStyle.Render("esc") + components.HelpDescStyle.Render(" back  ") +
				components.HelpKeyStyle.Render("d") + components.HelpDescStyle.Render(" delete  ") +
				components.HelpKeyStyle.Render("j/k") + components.HelpDescStyle.Render(" scroll  ") +
				components.HelpKeyStyle.Render("q") + components.HelpDescStyle.Render(" quit")
		} else {
			// List view help
			selectedInfo := ""
			if count := a.selectedCount(); count > 0 {
				selectedInfo = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("#10B981")).
					Render(fmt.Sprintf(" %d selected ", count))
			}

			help = components.HelpKeyStyle.Render("enter") + components.HelpDescStyle.Render(" read  ") +
				components.HelpKeyStyle.Render("space") + components.HelpDescStyle.Render(" toggle  ") +
				components.HelpKeyStyle.Render("a") + components.HelpDescStyle.Render(" all  ") +
				components.HelpKeyStyle.Render("d") + components.HelpDescStyle.Render(" delete  ") +
				components.HelpKeyStyle.Render("e") + components.HelpDescStyle.Render(" archive  ") +
				components.HelpKeyStyle.Render("r") + components.HelpDescStyle.Render(" mark read  ") +
				components.HelpKeyStyle.Render("q") + components.HelpDescStyle.Render(" quit") +
				selectedInfo
		}
	default:
		help = ""
	}

	// Show message if available, otherwise show email count
	var status string
	if a.message != "" && a.state == searchStateReady {
		status = components.SuccessStyle.Render(a.message)
	} else {
		status = components.StatusKeyStyle.Render(fmt.Sprintf("%d emails", len(a.emails)))
	}

	gap := a.width - lipgloss.Width(help) - lipgloss.Width(status) - 4
	if gap < 0 {
		gap = 0
	}

	return components.StatusBarStyle.Width(a.width).Render(
		help + strings.Repeat(" ", gap) + status,
	)
}

// Helper functions
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func extractNameFromEmail(from string) string {
	if idx := strings.Index(from, "<"); idx > 0 {
		return strings.TrimSpace(from[:idx])
	}
	return from
}

func formatEmailDate(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	if t.Year() == now.Year() {
		return t.Format("Jan 02")
	}
	return t.Format("02/01/06")
}
