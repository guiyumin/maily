package ui

import (
	"fmt"
	"strings"
	"maily/internal/ui/utils"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/emersion/go-imap/v2"

	"maily/internal/auth"
	"maily/internal/mail"
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
	actionMarkRead
)

type searchView int

const (
	searchListView searchView = iota
	searchReadView
)

// confirmOption represents the selected button in confirmation dialogs
type confirmOption int

const (
	confirmOptionYes confirmOption = iota
	confirmOptionNo
)

type SearchApp struct {
	account             *auth.Account
	query               string
	imap                *mail.IMAPClient
	uids                []imap.UID       // All matching UIDs from search
	emails              map[int]mail.Email // Lazily loaded emails by index
	selected            map[int]bool
	cursor              int
	state               searchState
	view                searchView
	action              actionType
	spinner             spinner.Model
	viewport            viewport.Model
	width               int
	height              int
	err                 error
	message             string
	scrollCount         int
	confirmDeleteSingle bool
	confirmSelection    confirmOption // Selected button in confirm dialogs
	loadingRange        bool          // True when fetching a new range
	lastLoadedStart     int           // Start of last loaded range
	lastLoadedEnd       int           // End of last loaded range
}

// searchUIDsMsg is sent when UIDs are fetched (first phase)
type searchUIDsMsg struct {
	uids []imap.UID
}

// searchRangeMsg is sent when a range of emails is loaded (lazy loading)
type searchRangeMsg struct {
	start  int
	emails []mail.Email
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
		emails:   make(map[int]mail.Email),
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
		// Phase 1: Only fetch UIDs (fast)
		uids, err := mail.Search(&a.account.Credentials, "INBOX", a.query)
		if err != nil {
			return searchErrorMsg{err: err}
		}

		return searchUIDsMsg{uids: uids}
	}
}

// loadRange fetches email details for a range of UIDs
func (a *SearchApp) loadRange(start, end int) tea.Cmd {
	return func() tea.Msg {
		if start >= len(a.uids) {
			return searchRangeMsg{start: start, emails: nil}
		}
		if end > len(a.uids) {
			end = len(a.uids)
		}

		// Get UIDs for this range
		rangeUIDs := a.uids[start:end]
		if len(rangeUIDs) == 0 {
			return searchRangeMsg{start: start, emails: nil}
		}

		// Fetch email details
		client, err := mail.NewIMAPClient(&a.account.Credentials)
		if err != nil {
			return searchErrorMsg{err: err}
		}
		defer client.Close()

		emails, err := client.FetchByUIDs("INBOX", rangeUIDs)
		if err != nil {
			return searchErrorMsg{err: err}
		}

		return searchRangeMsg{start: start, emails: emails}
	}
}

func (a *SearchApp) executeAction() tea.Cmd {
	return func() tea.Msg {
		var uids []imap.UID
		for i := range a.selected {
			if a.selected[i] && i < len(a.uids) {
				uids = append(uids, a.uids[i])
			}
		}

		var err error
		switch a.action {
		case actionDelete:
			err = a.imap.DeleteMessages(uids)
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

	case searchUIDsMsg:
		a.uids = msg.uids
		if len(a.uids) == 0 {
			a.message = "No emails found matching your query."
			a.state = searchStateDone
			return a, nil
		}
		// Load first visible range
		a.loadingRange = true
		return a, a.loadRange(0, 50) // Load first 50 emails

	case searchRangeMsg:
		a.loadingRange = false
		// Store loaded emails in map
		for i, email := range msg.emails {
			a.emails[msg.start+i] = email
		}
		a.lastLoadedStart = msg.start
		a.lastLoadedEnd = msg.start + len(msg.emails)

		// First load - transition to ready state and create action client
		if a.state == searchStateLoading {
			a.state = searchStateReady
			// Create IMAP client for later actions
			client, err := mail.NewIMAPClient(&a.account.Credentials)
			if err != nil {
				a.state = searchStateError
				a.err = fmt.Errorf("failed to connect for actions: %w", err)
				return a, nil
			}
			client.SelectMailbox("INBOX")
			a.imap = client
		}

	case searchErrorMsg:
		a.state = searchStateError
		a.err = msg.err

	case actionCompleteMsg:
		actionName := ""
		switch a.action {
		case actionDelete:
			actionName = "deleted"
			// Remove deleted UIDs and rebuild emails map
			var remainingUIDs []imap.UID
			newEmails := make(map[int]mail.Email)
			newIdx := 0
			for i, uid := range a.uids {
				if !a.selected[i] {
					remainingUIDs = append(remainingUIDs, uid)
					if email, ok := a.emails[i]; ok {
						newEmails[newIdx] = email
					}
					newIdx++
				}
			}
			a.uids = remainingUIDs
			a.emails = newEmails
		case actionMarkRead:
			actionName = "marked as read"
			// Update unread status in the map
			for i := range a.selected {
				if a.selected[i] {
					if email, ok := a.emails[i]; ok {
						email.Unread = false
						a.emails[i] = email
					}
				}
			}
		}

		// Clear selections and reset state
		a.selected = make(map[int]bool)
		a.action = actionNone

		// Adjust cursor if needed
		if a.cursor >= len(a.uids) && a.cursor > 0 {
			a.cursor = len(a.uids) - 1
		}

		// Go back to ready state (list view)
		a.state = searchStateReady
		a.message = fmt.Sprintf("Successfully %s %d email(s).", actionName, msg.count)

		// If no emails left, show done state
		if len(a.uids) == 0 {
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
				imapClient := a.imap
				uid := email.UID
				go func() {
					imapClient.MarkAsRead(uid)
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

	case "l": // Load more emails
		if !a.loadingRange && len(a.emails) < len(a.uids) {
			a.loadingRange = true
			start := len(a.emails)
			end := start + 50
			if end > len(a.uids) {
				end = len(a.uids)
			}
			return a, a.loadRange(start, end)
		}

	case " ": // Space to toggle selection
		a.selected[a.cursor] = !a.selected[a.cursor]
		if a.cursor < len(a.emails)-1 {
			a.cursor++
		}

	case "a": // Select all
		allSelected := len(a.selected) == len(a.uids)
		if allSelected {
			// Deselect all
			a.selected = make(map[int]bool)
		} else {
			// Select all
			for i := range a.uids {
				a.selected[i] = true
			}
		}

	case "d": // Delete
		if a.selectedCount() > 0 {
			a.action = actionDelete
			a.state = searchStateConfirm
			a.confirmSelection = confirmOptionYes
		}

	case "r": // Mark as read
		if a.selectedCount() > 0 {
			a.action = actionMarkRead
			a.state = searchStateConfirm
			a.confirmSelection = confirmOptionYes
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
		// Delete current email - show confirmation
		if !a.confirmDeleteSingle {
			a.confirmDeleteSingle = true
			a.confirmSelection = confirmOptionYes // Default to Yes
		}

	case "left", "h":
		if a.confirmDeleteSingle {
			a.confirmSelection = confirmOptionYes
		}

	case "right", "l":
		if a.confirmDeleteSingle {
			a.confirmSelection = confirmOptionNo
		}

	case "enter":
		if a.confirmDeleteSingle {
			if a.confirmSelection == confirmOptionYes {
				// Delete the current email
				if a.cursor < len(a.uids) {
					uid := a.uids[a.cursor]
					// Remove UID from list and rebuild emails map
					newUIDs := append(a.uids[:a.cursor], a.uids[a.cursor+1:]...)
					newEmails := make(map[int]mail.Email)
					for i, u := range newUIDs {
						// Find the old index for this UID
						for oldIdx, oldUID := range a.uids {
							if oldUID == u {
								if email, ok := a.emails[oldIdx]; ok {
									newEmails[i] = email
								}
								break
							}
						}
					}
					a.uids = newUIDs
					a.emails = newEmails
					// Adjust cursor if needed
					if a.cursor >= len(a.uids) && a.cursor > 0 {
						a.cursor--
					}
					// Delete in background
					if a.imap != nil {
						imapClient := a.imap
						go func() {
							imapClient.DeleteMessage(uid)
						}()
					}
					// Go back to list view
					a.view = searchListView
					a.confirmDeleteSingle = false
				}
			} else {
				// Cancel
				a.confirmDeleteSingle = false
			}
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

func (a SearchApp) renderEmailContent(email mail.Email) string {
	body := email.Body
	if body == "" {
		body = email.Snippet
	}
	return body
}

func (a SearchApp) handleConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		a.confirmSelection = confirmOptionYes

	case "right", "l":
		a.confirmSelection = confirmOptionNo

	case "enter":
		if a.confirmSelection == confirmOptionYes {
			a.state = searchStateExecuting
			return a, tea.Batch(a.spinner.Tick, a.executeAction())
		} else {
			a.state = searchStateReady
			a.action = actionNone
		}

	case "esc":
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
		selectedStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1F2937")).
			Background(lipgloss.Color("#EF4444")).
			Padding(0, 2)

		unselectedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")).
			Padding(0, 2)

		var yesBtn, noBtn string
		if a.confirmSelection == confirmOptionYes {
			yesBtn = selectedStyle.Render("Yes")
			noBtn = unselectedStyle.Render("No")
		} else {
			yesBtn = unselectedStyle.Render("Yes")
			noBtn = selectedStyle.Background(lipgloss.Color("#6B7280")).Render("No")
		}

		buttons := lipgloss.JoinHorizontal(lipgloss.Center, yesBtn, "  ", noBtn)

		hint := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Render("← → to select, enter to confirm, esc to cancel")

		confirmDialog := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#EF4444")).
			Padding(1, 3).
			Align(lipgloss.Center).
			Render(
				lipgloss.JoinVertical(lipgloss.Center,
					lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444")).Render("Delete this email?"),
					"",
					buttons,
					"",
					hint,
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

func (a SearchApp) renderEmailLine(email mail.Email, cursor bool, selected bool) string {
	maxWidth := a.width - 17 // Account for checkbox, status, attachment icon
	if maxWidth < 40 {
		maxWidth = 80
	}

	from := utils.TruncateStr(utils.ExtractNameFromEmail(email.From), 20)
	subject := utils.TruncateStr(email.Subject, maxWidth-35)
	date := utils.FormatEmailDate(email.Date)

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

	// Attachment indicator
	var attachIcon string
	if len(email.Attachments) > 0 {
		attachIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render("📎")
	} else {
		attachIcon = "  " // Same width placeholder
	}

	line := fmt.Sprintf("%-20s │ %-*s │ %s",
		from,
		maxWidth-35,
		subject,
		date,
	)

	lineStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Width(a.width - 17)

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

	return checkbox + status + attachIcon + " " + lineStyle.Render(line)
}

func (a SearchApp) renderConfirmDialog() string {
	actionName := ""
	actionColor := lipgloss.Color("#EF4444")

	switch a.action {
	case actionDelete:
		actionName = "Delete"
		actionColor = lipgloss.Color("#EF4444")
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

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#1F2937")).
		Background(actionColor).
		Padding(0, 2)

	unselectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Padding(0, 2)

	var yesBtn, noBtn string
	if a.confirmSelection == confirmOptionYes {
		yesBtn = selectedStyle.Render("Yes")
		noBtn = unselectedStyle.Render("No")
	} else {
		yesBtn = unselectedStyle.Render("Yes")
		noBtn = selectedStyle.Background(lipgloss.Color("#6B7280")).Render("No")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, yesBtn, "  ", noBtn)

	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Render("← → to select, enter to confirm, esc to cancel")

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
				buttons,
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

			// Show load more hint if there are more emails
			loadMoreHint := ""
			if len(a.emails) < len(a.uids) {
				loadMoreHint = components.HelpKeyStyle.Render("l") + components.HelpDescStyle.Render(" load more  ")
			}

			help = components.HelpKeyStyle.Render("enter") + components.HelpDescStyle.Render(" read  ") +
				components.HelpKeyStyle.Render("space") + components.HelpDescStyle.Render(" toggle  ") +
				components.HelpKeyStyle.Render("a") + components.HelpDescStyle.Render(" all  ") +
				loadMoreHint +
				components.HelpKeyStyle.Render("d") + components.HelpDescStyle.Render(" delete  ") +
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
	} else if a.loadingRange {
		status = components.StatusKeyStyle.Render(fmt.Sprintf("Loading... %d/%d emails", len(a.emails), len(a.uids)))
	} else {
		status = components.StatusKeyStyle.Render(fmt.Sprintf("%d/%d emails", len(a.emails), len(a.uids)))
	}

	gap := a.width - lipgloss.Width(help) - lipgloss.Width(status) - 4
	if gap < 0 {
		gap = 0
	}

	return components.StatusBarStyle.Width(a.width).Render(
		help + strings.Repeat(" ", gap) + status,
	)
}

