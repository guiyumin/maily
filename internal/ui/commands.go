package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/emersion/go-imap/v2"
	"maily/internal/gmail"
)

type bulkActionCompleteMsg struct {
	action string
	count  int
}

type draftSavedMsg struct{}

type draftSaveErrorMsg struct {
	err error
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
	label := a.currentLabel
	return func() tea.Msg {
		emails, err := a.imap.FetchMessages(label, a.emailLimit)
		if err != nil {
			return errorMsg{err: err}
		}
		return emailsLoadedMsg{emails: emails}
	}
}

func (a *App) loadLabels() tea.Cmd {
	return func() tea.Msg {
		labels, err := a.imap.ListMailboxes()
		if err != nil {
			return errorMsg{err: err}
		}
		return labelsLoadedMsg{labels: labels}
	}
}

func (a *App) executeSearch(query string) tea.Cmd {
	label := a.currentLabel
	return func() tea.Msg {
		emails, err := a.imap.SearchMessages(label, query)
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

func (a *App) sendReply() tea.Cmd {
	account := a.currentAccount()
	if account == nil {
		return func() tea.Msg {
			return replySendErrorMsg{err: fmt.Errorf("no account configured")}
		}
	}

	to := a.compose.GetTo()
	subject := a.compose.GetSubject()
	body := a.compose.GetBody()
	original := a.compose.GetOriginalEmail()

	return func() tea.Msg {
		smtp := gmail.NewSMTPClient(&account.Credentials)

		var err error
		if original != nil {
			err = smtp.Reply(to, subject, body, original.MessageID, original.References)
		} else {
			err = smtp.Send(to, subject, body)
		}

		if err != nil {
			return replySendErrorMsg{err: err}
		}
		return replySentMsg{}
	}
}

func (a *App) saveDraft() tea.Cmd {
	to := a.compose.GetTo()
	subject := a.compose.GetSubject()
	body := a.compose.GetBody()

	return func() tea.Msg {
		if err := a.imap.SaveDraft(to, subject, body); err != nil {
			return draftSaveErrorMsg{err: err}
		}
		return draftSavedMsg{}
	}
}

// executeCommand handles slash command execution
func (a App) executeCommand(command string) (tea.Model, tea.Cmd) {
	switch command {
	case "compose":
		// Compose new email
		account := a.currentAccount()
		if account != nil {
			a.compose = NewComposeModel(account.Credentials.Email)
			a.compose.width = a.width
			a.compose.height = a.height
			a.view = composeView
			return a, a.compose.Init()
		}

	case "reply":
		// Reply to selected email
		if email := a.mailList.SelectedEmail(); email != nil {
			account := a.currentAccount()
			if account != nil {
				a.compose = NewReplyModel(account.Credentials.Email, email)
				a.compose.width = a.width
				a.compose.height = a.height
				a.view = composeView
				return a, a.compose.Init()
			}
		}

	case "delete":
		// Delete selected email
		if a.mailList.SelectedEmail() != nil {
			a.confirmDelete = true
		}

	case "search":
		// Enter search mode
		a.searchMode = true
		a.searchInput.Focus()
		return a, textinput.Blink

	case "refresh":
		// Refresh inbox
		if !a.isSearchResult {
			a.state = stateLoading
			a.statusMsg = "Refreshing..."
			return a, tea.Batch(a.spinner.Tick, a.loadEmails())
		}

	case "labels":
		// Show label picker
		if !a.isSearchResult && a.view == listView {
			a.labelPicker.SetSelected(a.currentLabel)
			a.showLabelPicker = true
		}

	case "summarize":
		// AI summarize (placeholder - will be implemented in Phase 1)
		a.statusMsg = "Summarize: AI not configured yet"

	case "extract":
		// AI extract event (placeholder - will be implemented in Phase 1)
		a.statusMsg = "Extract: AI not configured yet"

	case "add":
		// Add calendar event (placeholder)
		a.statusMsg = "Add event: Not implemented yet"
	}

	return a, nil
}
