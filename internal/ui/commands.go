package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/emersion/go-imap/v2"
	"maily/internal/gmail"
)

type bulkActionCompleteMsg struct {
	action string
	count  int
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

func (a *App) executeSearch(query string) tea.Cmd {
	return func() tea.Msg {
		emails, err := a.imap.SearchMessages("INBOX", query)
		if err != nil {
			return errorMsg{err: err}
		}
		return appSearchResultsMsg{emails: emails, query: query}
	}
}

func (a *App) markSelectedAsRead() tea.Cmd {
	// Collect UIDs of selected emails
	var uids []imap.UID
	for uid, selected := range a.selected {
		if selected {
			uids = append(uids, uid)
		}
	}

	return func() tea.Msg {
		if len(uids) == 0 {
			return bulkActionCompleteMsg{action: "marked as read", count: 0}
		}
		if err := a.imap.MarkMessagesAsRead(uids); err != nil {
			return errorMsg{err: err}
		}
		return bulkActionCompleteMsg{action: "marked as read", count: len(uids)}
	}
}

func (a *App) deleteSelectedEmails() tea.Cmd {
	// Collect UIDs of selected emails
	var uids []imap.UID
	for uid, selected := range a.selected {
		if selected {
			uids = append(uids, uid)
		}
	}

	return func() tea.Msg {
		if len(uids) == 0 {
			return bulkActionCompleteMsg{action: "deleted", count: 0}
		}
		if err := a.imap.DeleteMessages(uids); err != nil {
			return errorMsg{err: err}
		}
		return bulkActionCompleteMsg{action: "deleted", count: len(uids)}
	}
}
