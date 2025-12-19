package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/emersion/go-imap/v2"

	"maily/internal/auth"
	"maily/internal/gmail"
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

type SearchApp struct {
	account   *auth.Account
	query     string
	imap      *gmail.IMAPClient
	emails    []gmail.Email
	selected  map[int]bool
	cursor    int
	state     searchState
	action    actionType
	spinner   spinner.Model
	width     int
	height    int
	err       error
	message   string
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
	s.Style = SpinnerStyle

	return SearchApp{
		account:  account,
		query:    query,
		selected: make(map[int]bool),
		state:    searchStateLoading,
		spinner:  s,
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
		a.imap = client

	case searchErrorMsg:
		a.state = searchStateError
		a.err = msg.err

	case actionCompleteMsg:
		a.state = searchStateDone
		actionName := ""
		switch a.action {
		case actionDelete:
			actionName = "deleted"
		case actionArchive:
			actionName = "archived"
		case actionMarkRead:
			actionName = "marked as read"
		}
		a.message = fmt.Sprintf("Successfully %s %d email(s).", actionName, msg.count)
	}

	return a, nil
}

func (a SearchApp) handleReadyKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		if a.imap != nil {
			a.imap.Close()
		}
		return a, tea.Quit

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
		content = a.renderResults()

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
			SuccessStyle.Render(a.message+"\n\nPress Enter or q to exit."),
		)

	case searchStateError:
		content = lipgloss.Place(
			a.width,
			a.height-4,
			lipgloss.Center,
			lipgloss.Center,
			ErrorStyle.Render(fmt.Sprintf("Error: %v\n\nPress Enter or q to exit.", a.err)),
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
	title := TitleStyle.Render(" MAILY SEARCH ")

	queryInfo := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Render(fmt.Sprintf("Query: %s", a.query))

	return HeaderStyle.Width(a.width).Render(title + "  " + queryInfo)
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
		selectedInfo := ""
		if count := a.selectedCount(); count > 0 {
			selectedInfo = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#10B981")).
				Render(fmt.Sprintf(" %d selected ", count))
		}

		help = HelpKeyStyle.Render("space") + HelpDescStyle.Render(" toggle  ") +
			HelpKeyStyle.Render("a") + HelpDescStyle.Render(" all  ") +
			HelpKeyStyle.Render("d") + HelpDescStyle.Render(" delete  ") +
			HelpKeyStyle.Render("e") + HelpDescStyle.Render(" archive  ") +
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" mark read  ") +
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit") +
			selectedInfo
	default:
		help = ""
	}

	status := StatusKeyStyle.Render(fmt.Sprintf("%d emails", len(a.emails)))

	gap := a.width - lipgloss.Width(help) - lipgloss.Width(status) - 4
	if gap < 0 {
		gap = 0
	}

	return StatusBarStyle.Width(a.width).Render(
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
