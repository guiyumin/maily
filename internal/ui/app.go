package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cocomail/internal/auth"
	"cocomail/internal/gmail"
	"cocomail/internal/ui/components"
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

func NewApp(store *auth.AccountStore) App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	return App{
		store:      store,
		accountIdx: 0,
		imapCache:  make(map[int]*gmail.IMAPClient),
		emailCache: make(map[int][]gmail.Email),
		mailList:   components.NewMailList(),
		spinner:    s,
		state:      stateLoading,
		view:       listView,
		emailLimit: 50,
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

func (a App) initClient() tea.Cmd {
	account := a.currentAccount()
	if account == nil {
		return func() tea.Msg {
			return errorMsg{err: fmt.Errorf("no account configured")}
		}
	}
	creds := &account.Credentials
	return func() tea.Msg {
		client, err := gmail.NewIMAPClient(creds)
		if err != nil {
			return errorMsg{err: err}
		}
		return clientReadyMsg{imap: client}
	}
}

func (a *App) loadEmails() tea.Cmd {
	return func() tea.Msg {
		emails, err := a.imap.FetchMessages("INBOX", a.emailLimit)
		if err != nil {
			return errorMsg{err: err}
		}
		return emailsLoadedMsg{emails: emails}
	}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
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
				a.view = listView
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
			if a.state == stateReady {
				a.state = stateLoading
				a.statusMsg = "Refreshing..."
				return a, tea.Batch(a.spinner.Tick, a.loadEmails())
			}
		case "d":
			if a.state == stateReady && !a.confirmDelete {
				if a.mailList.SelectedEmail() != nil {
					a.confirmDelete = true
				}
			}
		case "y":
			if a.confirmDelete {
				if email := a.mailList.SelectedEmail(); email != nil {
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
			if a.view == listView && a.state == stateReady && !a.confirmDelete {
				a.emailLimit += 50
				a.state = stateLoading
				a.statusMsg = fmt.Sprintf("Loading %d emails...", a.emailLimit)
				return a, tea.Batch(a.spinner.Tick, a.loadEmails())
			}
		case "tab":
			if len(a.store.Accounts) > 1 && !a.confirmDelete {
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
				if a.view == listView {
					a.mailList.ScrollUp()
				} else if a.view == readView {
					a.viewport.LineUp(3)
				}
			case tea.MouseButtonWheelDown:
				if a.view == listView {
					a.mailList.ScrollDown()
				} else if a.view == readView {
					a.viewport.LineDown(3)
				}
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.mailList.SetSize(msg.Width, msg.Height-6)
		a.viewport = viewport.New(msg.Width-4, msg.Height-8)
		a.viewport.Style = lipgloss.NewStyle().Padding(1, 2)

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
		content = lipgloss.Place(
			a.width,
			a.height-4,
			lipgloss.Center,
			lipgloss.Center,
			fmt.Sprintf("%s %s", a.spinner.View(), a.statusMsg),
		)
	case stateError:
		content = lipgloss.Place(
			a.width,
			a.height-4,
			lipgloss.Center,
			lipgloss.Center,
			ErrorStyle.Render(fmt.Sprintf("Error: %v", a.err)),
		)
	case stateReady:
		switch a.view {
		case listView:
			content = a.renderListView()
		case readView:
			content = a.renderReadView()
		}
	}

	// Show confirmation dialog overlay
	if a.confirmDelete {
		dialog := a.renderConfirmDialog()
		content = lipgloss.Place(
			a.width,
			a.height-4,
			lipgloss.Center,
			lipgloss.Center,
			dialog,
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		a.renderHeader(),
		content,
		a.renderStatusBar(),
	)
}

func (a App) renderConfirmDialog() string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#EF4444")).
		Padding(1, 3).
		Align(lipgloss.Center)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#EF4444")).
		Render("Delete Email?")

	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Render("press y to confirm, n to cancel")

	return dialogStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			title,
			"",
			hint,
		),
	)
}

func (a App) renderHeader() string {
	title := TitleStyle.Render(" COCOMAIL ")

	// Render account tabs
	var tabs []string
	activeTabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Background(lipgloss.Color("#7C3AED")).
		Padding(0, 1)
	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Padding(0, 1)

	for i, acc := range a.store.Accounts {
		email := acc.Credentials.Email
		// Shorten email for display
		if at := strings.Index(email, "@"); at > 0 {
			email = email[:at]
		}
		if i == a.accountIdx {
			tabs = append(tabs, activeTabStyle.Render(email))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(email))
		}
	}

	tabsStr := strings.Join(tabs, " ")
	return HeaderStyle.Width(a.width).Render(title + " " + tabsStr)
}

func (a App) renderListView() string {
	return lipgloss.NewStyle().
		Width(a.width).
		Height(a.height - 6).
		Render(a.mailList.View())
}

func (a App) renderReadView() string {
	email := a.mailList.SelectedEmail()
	if email == nil {
		return ""
	}

	header := lipgloss.JoinVertical(
		lipgloss.Left,
		FromStyle.Render("From: ")+email.From,
		"To: "+email.To,
		SubjectStyle.Render("Subject: ")+email.Subject,
		DateStyle.Render(email.Date.Format("Mon, 02 Jan 2006 15:04:05")),
		strings.Repeat("─", a.width-4),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		a.viewport.View(),
	)
}

func (a App) renderEmailContent(email gmail.Email) string {
	body := email.Body
	if body == "" {
		body = email.Snippet
	}
	return body
}

func (a App) renderStatusBar() string {
	var help string
	tabHint := ""
	if len(a.store.Accounts) > 1 {
		tabHint = HelpKeyStyle.Render("tab") + HelpDescStyle.Render(" switch  ")
	}
	if a.view == listView {
		help = tabHint +
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" navigate  ") +
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" open  ") +
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" refresh  ") +
			HelpKeyStyle.Render("d") + HelpDescStyle.Render(" delete  ") +
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit")
	} else {
		help = tabHint +
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back  ") +
			HelpKeyStyle.Render("j/k") + HelpDescStyle.Render(" scroll  ") +
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit")
	}

	status := StatusKeyStyle.Render(a.statusMsg)

	gap := a.width - lipgloss.Width(help) - lipgloss.Width(status) - 4
	if gap < 0 {
		gap = 0
	}

	return StatusBarStyle.Width(a.width).Render(
		help + strings.Repeat(" ", gap) + status,
	)
}
