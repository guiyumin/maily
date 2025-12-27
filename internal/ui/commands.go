package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/emersion/go-imap/v2"
	"maily/internal/ai"
	"maily/internal/cache"
	"maily/internal/mail"
)

const (
	// SyncDays matches the sync window in internal/sync
	SyncDays = 14
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
	accountEmail := creds.Email // capture for closure
	return func() tea.Msg {
		client, err := mail.NewIMAPClient(creds)
		if err != nil {
			return errorMsg{err: err, accountEmail: accountEmail}
		}
		return clientReadyMsg{imap: client}
	}
}

func (a *App) loadEmails() tea.Cmd {
	label := a.currentLabel
	accountEmail := ""
	if account := a.currentAccount(); account != nil {
		accountEmail = account.Credentials.Email
	}
	since := time.Now().AddDate(0, 0, -SyncDays)
	return func() tea.Msg {
		emails, err := a.imap.FetchMessagesSince(label, since, a.emailLimit)
		if err != nil {
			return errorMsg{err: err, accountEmail: accountEmail}
		}
		return emailsLoadedMsg{emails: emails}
	}
}

func (a *App) loadLabels() tea.Cmd {
	accountEmail := ""
	if account := a.currentAccount(); account != nil {
		accountEmail = account.Credentials.Email
	}
	return func() tea.Msg {
		labels, err := a.imap.ListMailboxes()
		if err != nil {
			return errorMsg{err: err, accountEmail: accountEmail}
		}
		return labelsLoadedMsg{labels: labels}
	}
}

func (a *App) executeSearch(query string) tea.Cmd {
	label := a.currentLabel
	accountEmail := ""
	if account := a.currentAccount(); account != nil {
		accountEmail = account.Credentials.Email
	}
	return func() tea.Msg {
		emails, err := a.imap.SearchMessages(label, query)
		if err != nil {
			return errorMsg{err: err, accountEmail: accountEmail}
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

	account := a.currentAccount()
	accountEmail := ""
	if account != nil {
		accountEmail = account.Credentials.Email
	}
	mailbox := a.currentLabel
	diskCache := a.diskCache

	return func() tea.Msg {
		if len(uids) == 0 {
			return bulkActionCompleteMsg{action: "marked as read", count: 0}
		}
		if err := a.imap.MarkMessagesAsRead(uids); err != nil {
			return errorMsg{err: err, accountEmail: accountEmail}
		}
		// Update disk cache
		if diskCache != nil && account != nil {
			for _, uid := range uids {
				diskCache.UpdateEmailFlags(accountEmail, mailbox, uid, false)
			}
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

	account := a.currentAccount()
	accountEmail := ""
	if account != nil {
		accountEmail = account.Credentials.Email
	}
	mailbox := a.currentLabel
	diskCache := a.diskCache

	return func() tea.Msg {
		if len(uids) == 0 {
			return bulkActionCompleteMsg{action: "deleted", count: 0}
		}
		err := a.imap.DeleteMessages(uids)
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to delete: %w", err), accountEmail: accountEmail}
		}

		// Only remove from disk cache if server delete succeeded
		if diskCache != nil && accountEmail != "" {
			for _, uid := range uids {
				diskCache.DeleteEmail(accountEmail, mailbox, uid)
			}
		}
		return bulkActionCompleteMsg{action: "deleted", count: len(uids)}
	}
}

func (a *App) deleteSingleEmail(uid imap.UID) tea.Cmd {
	account := a.currentAccount()
	accountEmail := ""
	if account != nil {
		accountEmail = account.Credentials.Email
	}
	mailbox := a.currentLabel
	diskCache := a.diskCache

	return func() tea.Msg {
		err := a.imap.DeleteMessage(uid)
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to delete: %w", err), accountEmail: accountEmail}
		}

		// Only remove from disk cache if server delete succeeded
		if diskCache != nil && accountEmail != "" {
			diskCache.DeleteEmail(accountEmail, mailbox, uid)
		}
		return singleDeleteCompleteMsg{uid: uid}
	}
}

func (a *App) moveSelectedToTrash() tea.Cmd {
	// Collect UIDs of selected emails
	var uids []imap.UID
	for uid, selected := range a.selected {
		if selected {
			uids = append(uids, uid)
		}
	}

	account := a.currentAccount()
	accountEmail := ""
	if account != nil {
		accountEmail = account.Credentials.Email
	}
	mailbox := a.currentLabel
	diskCache := a.diskCache

	return func() tea.Msg {
		if len(uids) == 0 {
			return bulkActionCompleteMsg{action: "moved to trash", count: 0}
		}
		err := a.imap.MoveToTrash(uids)
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to move to trash: %w", err), accountEmail: accountEmail}
		}

		// Only remove from disk cache if server move succeeded
		if diskCache != nil && accountEmail != "" {
			for _, uid := range uids {
				diskCache.DeleteEmail(accountEmail, mailbox, uid)
			}
		}
		return bulkActionCompleteMsg{action: "moved to trash", count: len(uids)}
	}
}

func (a *App) moveSingleToTrash(uid imap.UID) tea.Cmd {
	account := a.currentAccount()
	accountEmail := ""
	if account != nil {
		accountEmail = account.Credentials.Email
	}
	mailbox := a.currentLabel
	diskCache := a.diskCache

	return func() tea.Msg {
		err := a.imap.MoveToTrash([]imap.UID{uid})
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to move to trash: %w", err), accountEmail: accountEmail}
		}

		// Only remove from disk cache if server move succeeded
		if diskCache != nil && accountEmail != "" {
			diskCache.DeleteEmail(accountEmail, mailbox, uid)
		}
		return singleDeleteCompleteMsg{uid: uid}
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
		smtp := mail.NewSMTPClient(&account.Credentials)

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

func (a *App) summarizeEmail(email *mail.Email) tea.Cmd {
	client := a.aiClient
	body := email.Body
	if body == "" {
		body = email.Snippet
	}
	prompt := ai.SummarizePrompt(email.From, email.Subject, body)
	provider := client.Provider()

	return func() tea.Msg {
		summary, err := client.Call(prompt)
		if err != nil {
			return summaryErrorMsg{err: err}
		}
		return summaryResultMsg{summary: summary, provider: provider}
	}
}

// loadCachedEmails loads emails from disk cache for instant display
func (a App) loadCachedEmails() tea.Cmd {
	account := a.currentAccount()
	if account == nil || a.diskCache == nil {
		return nil
	}

	email := account.Credentials.Email
	mailbox := a.currentLabel

	return func() tea.Msg {
		cached, err := a.diskCache.LoadEmailsLimit(email, mailbox, 50)
		if err != nil || len(cached) == 0 {
			return cachedEmailsLoadedMsg{emails: nil}
		}

		// Convert cached emails to mail.Email format
		emails := make([]mail.Email, len(cached))
		for i, c := range cached {
			emails[i] = cachedToGmail(c)
		}
		return cachedEmailsLoadedMsg{emails: emails}
	}
}

// reloadFromCache reloads emails from disk cache (for manual refresh)
func (a App) reloadFromCache() tea.Cmd {
	account := a.currentAccount()
	if account == nil || a.diskCache == nil {
		return func() tea.Msg {
			return emailsLoadedMsg{emails: nil}
		}
	}

	accountEmail := account.Credentials.Email
	mailbox := a.currentLabel
	limit := int(a.emailLimit)

	return func() tea.Msg {
		cached, err := a.diskCache.LoadEmailsLimit(accountEmail, mailbox, limit)
		if err != nil {
			return errorMsg{err: err, accountEmail: accountEmail}
		}

		// Convert cached emails to mail.Email format
		emails := make([]mail.Email, len(cached))
		for i, c := range cached {
			emails[i] = cachedToGmail(c)
		}
		return emailsLoadedMsg{emails: emails}
	}
}

// cachedToGmail converts a cache.CachedEmail to mail.Email
func cachedToGmail(c cache.CachedEmail) mail.Email {
	attachments := make([]mail.Attachment, len(c.Attachments))
	for i, a := range c.Attachments {
		attachments[i] = mail.Attachment{
			PartID:      a.PartID,
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Size:        a.Size,
		}
	}

	return mail.Email{
		UID:          c.UID,
		MessageID:    c.MessageID,
		InternalDate: c.InternalDate,
		From:         c.From,
		ReplyTo:      c.ReplyTo,
		To:           c.To,
		Subject:      c.Subject,
		Date:         c.Date,
		Snippet:      c.Snippet,
		Body:         c.Body,
		Unread:       c.Unread,
		References:   c.References,
		Attachments:  attachments,
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
		// Refresh from IMAP server
		if !a.isSearchResult && a.view == listView {
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
		// AI summarize
		if a.view == readView {
			if !a.aiClient.Available() {
				a.statusMsg = "No AI CLI found (install claude, codex, gemini, vibe, or ollama)"
				return a, nil
			}
			if email := a.mailList.SelectedEmail(); email != nil {
				a.state = stateLoading
				a.statusMsg = "Summarizing with " + a.aiClient.Provider() + "..."
				return a, tea.Batch(a.spinner.Tick, a.summarizeEmail(email))
			}
		}

	case "extract":
		// AI extract event (placeholder - will be implemented in Phase 1)
		a.statusMsg = "Extract: AI not configured yet"

	case "add":
		// Add calendar event (placeholder)
		a.statusMsg = "Add event: Not implemented yet"
	}

	return a, nil
}
