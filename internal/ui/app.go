package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/emersion/go-imap/v2"

	"maily/internal/auth"
	"maily/internal/gmail"
	"maily/internal/ui/components"
)

type selectedEmail struct {
	email gmail.Email
}

type view int

const (
	listView view = iota
	readView
	composeView
)

type state int

const (
	stateLoading state = iota
	stateReady
	stateError
)

type App struct {
	store         *auth.AccountStore
	accountIdx    int
	imap          *gmail.IMAPClient
	imapCache     map[int]*gmail.IMAPClient
	emailCache    map[int][]gmail.Email
	mailList      components.MailList
	viewport      viewport.Model
	spinner       spinner.Model
	state         state
	view          view
	width         int
	height        int
	err           error
	statusMsg     string
	confirmDelete bool
	emailLimit    uint32

	// Search
	searchInput    textinput.Model
	searchMode     bool // typing search query
	isSearchResult bool // showing search results
	searchQuery    string
	inboxCache     []gmail.Email

	// Multi-select (search mode only)
	selected map[imap.UID]bool

	// Scroll throttling (count-based)
	scrollCount int
}

type emailsLoadedMsg struct {
	emails []gmail.Email
}

type errorMsg struct {
	err error
}

type clientReadyMsg struct {
	imap *gmail.IMAPClient
}

type appSearchResultsMsg struct {
	emails []gmail.Email
	query  string
}

func NewApp(store *auth.AccountStore) App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = components.SpinnerStyle

	si := textinput.New()
	si.Placeholder = "Search emails..."
	si.CharLimit = 200
	si.Width = 40

	vp := viewport.New(80, 24) // Default size, will be resized by WindowSizeMsg
	vp.Style = lipgloss.NewStyle().Padding(1, 4, 3, 4)

	return App{
		store:       store,
		accountIdx:  0,
		imapCache:   make(map[int]*gmail.IMAPClient),
		emailCache:  make(map[int][]gmail.Email),
		mailList:    components.NewMailList(),
		viewport:    vp,
		spinner:     s,
		state:       stateLoading,
		view:        listView,
		emailLimit:  50,
		searchInput: si,
		selected:    make(map[imap.UID]bool),
	}
}

func (a App) currentAccount() *auth.Account {
	if a.accountIdx >= 0 && a.accountIdx < len(a.store.Accounts) {
		return &a.store.Accounts[a.accountIdx]
	}
	return nil
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.spinner.Tick,
		a.initClient(),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle search mode input
		if a.searchMode {
			switch msg.String() {
			case "esc":
				a.searchMode = false
				a.searchInput.Blur()
				a.searchInput.SetValue("")
			case "enter":
				query := a.searchInput.Value()
				if query != "" {
					a.searchMode = false
					a.searchInput.Blur()
					a.state = stateLoading
					a.statusMsg = "Searching..."
					// Cache inbox before search
					if !a.isSearchResult {
						a.inboxCache = a.mailList.Emails()
					}
					return a, tea.Batch(a.spinner.Tick, a.executeSearch(query))
				}
			default:
				var cmd tea.Cmd
				a.searchInput, cmd = a.searchInput.Update(msg)
				return a, cmd
			}
			return a, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			// Close all cached IMAP connections
			for _, client := range a.imapCache {
				if client != nil {
					client.Close()
				}
			}
			if a.imap != nil {
				a.imap.Close()
			}
			return a, tea.Quit
		case "esc":
			if a.confirmDelete {
				a.confirmDelete = false
				a.statusMsg = ""
			} else if a.view == readView {
				// Go back to list view (preserves search mode if active)
				a.view = listView
			} else if a.isSearchResult {
				// Exit search results, refresh inbox to reflect any deletions
				a.isSearchResult = false
				a.searchQuery = ""
				a.searchInput.SetValue("")
				a.selected = make(map[imap.UID]bool) // Clear selections
				a.mailList.SetSelectionMode(false)
				a.state = stateLoading
				a.statusMsg = "Refreshing..."
				return a, tea.Batch(a.spinner.Tick, a.loadEmails())
			}
		case "/":
			if a.view == listView && a.state == stateReady && !a.confirmDelete {
				a.searchMode = true
				a.searchInput.Focus()
				return a, textinput.Blink
			}
		case "enter":
			if a.view == listView && a.state == stateReady {
				if email := a.mailList.SelectedEmail(); email != nil {
					a.view = readView
					a.viewport.SetContent(a.renderEmailContent(*email))
					a.viewport.GotoTop()

					if email.Unread {
						go func() {
							a.imap.MarkAsRead(email.UID)
						}()
					}
				}
			}
		case "r":
			if a.state == stateReady && !a.isSearchResult {
				a.state = stateLoading
				a.statusMsg = "Refreshing..."
				return a, tea.Batch(a.spinner.Tick, a.loadEmails())
			}
		case "d":
			if a.state == stateReady && !a.confirmDelete {
				// In search mode with selections, delete selected emails
				if a.isSearchResult && a.selectedCount() > 0 {
					a.confirmDelete = true
				} else if a.mailList.SelectedEmail() != nil {
					a.confirmDelete = true
				}
			}
		case "y":
			if a.confirmDelete {
				// In search mode with selections, delete selected emails
				if a.isSearchResult && a.selectedCount() > 0 {
					a.state = stateLoading
					a.statusMsg = "Deleting..."
					a.confirmDelete = false
					return a, tea.Batch(a.spinner.Tick, a.deleteSelectedEmails())
				} else if email := a.mailList.SelectedEmail(); email != nil {
					uid := email.UID
					a.mailList.RemoveCurrent()
					a.view = listView
					go func() {
						a.imap.DeleteMessage(uid)
					}()
				}
				a.confirmDelete = false
				a.statusMsg = ""
			}
		case "n":
			if a.confirmDelete {
				a.confirmDelete = false
				a.statusMsg = ""
			}
		case "l":
			if a.view == listView && a.state == stateReady && !a.confirmDelete && !a.isSearchResult {
				a.emailLimit += 50
				a.state = stateLoading
				a.statusMsg = fmt.Sprintf("Loading %d emails...", a.emailLimit)
				return a, tea.Batch(a.spinner.Tick, a.loadEmails())
			}
		case " ": // Space to toggle selection (search mode only)
			if a.isSearchResult && a.view == listView && a.state == stateReady {
				if email := a.mailList.SelectedEmail(); email != nil {
					a.selected[email.UID] = !a.selected[email.UID]
					a.mailList.SetSelections(a.selected)
					// Move cursor down after selection
					if a.mailList.Cursor() < len(a.mailList.Emails())-1 {
						a.mailList.ScrollDown()
					}
				}
			}
		case "a": // Select/deselect all (search mode only)
			if a.isSearchResult && a.view == listView && a.state == stateReady {
				emails := a.mailList.Emails()
				allSelected := len(a.selected) == len(emails) && len(emails) > 0
				// Check if all are actually selected
				for _, email := range emails {
					if !a.selected[email.UID] {
						allSelected = false
						break
					}
				}
				if allSelected {
					// Deselect all
					a.selected = make(map[imap.UID]bool)
				} else {
					// Select all
					for _, email := range emails {
						a.selected[email.UID] = true
					}
				}
				a.mailList.SetSelections(a.selected)
			}
		case "m": // Mark read/unread (search mode only, for selected emails)
			if a.isSearchResult && a.view == listView && a.state == stateReady && a.selectedCount() > 0 {
				a.state = stateLoading
				a.statusMsg = "Marking as read..."
				return a, tea.Batch(a.spinner.Tick, a.markSelectedAsRead())
			}
		case "tab":
			if len(a.store.Accounts) > 1 && !a.confirmDelete && !a.isSearchResult {
				// Save current emails to cache
				if emails := a.mailList.Emails(); len(emails) > 0 {
					a.emailCache[a.accountIdx] = emails
				}
				// Save current IMAP connection to cache
				if a.imap != nil {
					a.imapCache[a.accountIdx] = a.imap
				}

				// Switch to next account
				a.accountIdx = (a.accountIdx + 1) % len(a.store.Accounts)
				a.view = listView

				// Check if we have cached data for this account
				if cached, ok := a.emailCache[a.accountIdx]; ok && len(cached) > 0 {
					a.imap = a.imapCache[a.accountIdx]
					a.mailList.SetEmails(cached)
					a.state = stateReady
					a.statusMsg = fmt.Sprintf("%d emails", len(cached))
					return a, nil
				}

				// No cache, need to load
				a.imap = nil
				a.state = stateLoading
				a.emailLimit = 50
				a.mailList.SetEmails(nil)
				a.statusMsg = "Loading..."
				return a, tea.Batch(a.spinner.Tick, a.initClient())
			}
		}

	case tea.MouseMsg:
		if a.state == stateReady && !a.confirmDelete {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				switch a.view {
				case listView:
					// Only process every 3rd scroll event
					a.scrollCount++
					if a.scrollCount >= 3 {
						a.mailList.ScrollUp()
						a.scrollCount = 0
					}
					return a, nil
				case readView:
					a.viewport.ScrollUp(3)
					return a, nil
				}
			case tea.MouseButtonWheelDown:
				switch a.view {
				case listView:
					// Only process every 3rd scroll event
					a.scrollCount++
					if a.scrollCount >= 3 {
						a.mailList.ScrollDown()
						a.scrollCount = 0
					}
					return a, nil
				case readView:
					a.viewport.ScrollDown(3)
					return a, nil
				}
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.mailList.SetSize(msg.Width, msg.Height-6)
		a.viewport.Width = msg.Width - 8
		a.viewport.Height = msg.Height - 8

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case clientReadyMsg:
		a.imap = msg.imap
		a.imapCache[a.accountIdx] = msg.imap
		a.statusMsg = "Loading emails..."
		return a, a.loadEmails()

	case emailsLoadedMsg:
		a.mailList.SetEmails(msg.emails)
		a.emailCache[a.accountIdx] = msg.emails
		a.state = stateReady
		a.statusMsg = fmt.Sprintf("%d emails", len(msg.emails))

	case errorMsg:
		a.state = stateError
		a.err = msg.err

	case appSearchResultsMsg:
		a.mailList.SetEmails(msg.emails)
		a.mailList.SetSelectionMode(true)
		a.mailList.SetSelections(a.selected)
		a.state = stateReady
		a.isSearchResult = true
		a.searchQuery = msg.query
		if len(msg.emails) == 0 {
			a.statusMsg = fmt.Sprintf("No results for '%s'", msg.query)
		} else {
			a.statusMsg = fmt.Sprintf("%d results for '%s'", len(msg.emails), msg.query)
		}

	case bulkActionCompleteMsg:
		a.state = stateReady
		// Clear selections after action
		a.selected = make(map[imap.UID]bool)
		a.mailList.SetSelections(a.selected)
		a.statusMsg = fmt.Sprintf("Successfully %s %d email(s)", msg.action, msg.count)
		// Re-run search to refresh the list
		if a.isSearchResult && a.searchQuery != "" {
			a.state = stateLoading
			return a, tea.Batch(a.spinner.Tick, a.executeSearch(a.searchQuery))
		}
	}

	if a.view == listView && a.state == stateReady {
		var cmd tea.Cmd
		a.mailList, cmd = a.mailList.Update(msg)
		cmds = append(cmds, cmd)
	}

	if a.view == readView {
		var cmd tea.Cmd
		a.viewport, cmd = a.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var content string

	switch a.state {
	case stateLoading:
		content = components.RenderLoading(a.width, a.height, a.spinner.View(), a.statusMsg)
	case stateError:
		content = components.RenderError(a.width, a.height, a.err)
	case stateReady:
		switch a.view {
		case listView:
			content = components.RenderListView(a.width, a.height, a.mailList.View())
		case readView:
			if email := a.mailList.SelectedEmail(); email != nil {
				emailData := components.EmailViewData{
					From:    email.From,
					To:      email.To,
					Subject: email.Subject,
					Date:    email.Date,
				}
				content = components.RenderReadView(emailData, a.width, a.viewport.View())
			}
		default:
			content = components.RenderListView(a.width, a.height, a.mailList.View())
		}
	}

	// Show confirmation dialog overlay
	if a.confirmDelete {
		deleteCount := 1
		if a.isSearchResult && a.selectedCount() > 0 {
			deleteCount = a.selectedCount()
		}
		content = components.RenderCentered(a.width, a.height, components.RenderConfirmDialog(deleteCount))
	}

	// Show search input overlay
	if a.searchMode {
		content = components.RenderCentered(a.width, a.height, components.RenderSearchInput(a.searchInput.View()))
	}

	// Build header data
	var accounts []string
	for _, acc := range a.store.Accounts {
		accounts = append(accounts, acc.Credentials.Email)
	}
	headerData := components.HeaderData{
		Width:          a.width,
		Accounts:       accounts,
		ActiveIdx:      a.accountIdx,
		IsSearchResult: a.isSearchResult,
		SearchQuery:    a.searchQuery,
	}

	// Build status bar data
	statusData := components.StatusBarData{
		Width:          a.width,
		StatusMsg:      a.statusMsg,
		SearchMode:     a.searchMode,
		IsSearchResult: a.isSearchResult,
		IsListView:     a.view == listView,
		AccountCount:   len(a.store.Accounts),
		SelectionCount: a.selectedCount(),
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		components.RenderHeader(headerData),
		content,
		components.RenderStatusBar(statusData),
	)
}

func (a App) renderEmailContent(email gmail.Email) string {
	body := email.Body
	if body == "" {
		body = email.Snippet
	}
	return body
}

func (a App) selectedCount() int {
	count := 0
	for _, selected := range a.selected {
		if selected {
			count++
		}
	}
	return count
}
