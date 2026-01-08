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
		return clientReadyMsg{imap: client, accountEmail: accountEmail}
	}
}

func (a *App) loadEmails() tea.Cmd {
	label := a.currentLabel
	accountEmail := ""
	if account := a.currentAccount(); account != nil {
		accountEmail = account.Credentials.Email
	}
	since := time.Now().AddDate(0, 0, -SyncDays)
	imapClient := a.imap // capture for nil check in closure
	return func() tea.Msg {
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		// Get UIDValidity for cache consistency with daemon
		mailboxInfo, err := imapClient.SelectMailboxWithInfo(label)
		if err != nil {
			return errorMsg{err: err, accountEmail: accountEmail}
		}
		emails, err := imapClient.FetchMessagesSince(label, since, a.emailLimit)
		if err != nil {
			return errorMsg{err: err, accountEmail: accountEmail}
		}
		return emailsLoadedMsg{emails: emails, accountEmail: accountEmail, uidValidity: mailboxInfo.UIDValidity}
	}
}

func (a *App) loadLabels() tea.Cmd {
	accountEmail := ""
	if account := a.currentAccount(); account != nil {
		accountEmail = account.Credentials.Email
	}
	imap := a.imap // capture for nil check in closure
	return func() tea.Msg {
		if imap == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		labels, err := imap.ListMailboxes()
		if err != nil {
			return errorMsg{err: err, accountEmail: accountEmail}
		}
		return labelsLoadedMsg{labels: labels, accountEmail: accountEmail}
	}
}

func (a *App) executeSearch(query string) tea.Cmd {
	label := a.currentLabel
	accountEmail := ""
	if account := a.currentAccount(); account != nil {
		accountEmail = account.Credentials.Email
	}
	imap := a.imap // capture for nil check in closure
	return func() tea.Msg {
		if imap == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		emails, err := imap.SearchMessages(label, query)
		if err != nil {
			return errorMsg{err: err, accountEmail: accountEmail}
		}
		return appSearchResultsMsg{emails: emails, query: query, accountEmail: accountEmail}
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
	imapClient := a.imap // capture for nil check in closure

	return func() tea.Msg {
		if len(uids) == 0 {
			return bulkActionCompleteMsg{action: "marked as read", count: 0}
		}
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		if err := imapClient.MarkMessagesAsRead(uids); err != nil {
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
	imapClient := a.imap // capture for nil check in closure

	return func() tea.Msg {
		if len(uids) == 0 {
			return bulkActionCompleteMsg{action: "deleted", count: 0}
		}
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		err := imapClient.DeleteMessages(uids)
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
	imapClient := a.imap // capture for nil check in closure

	return func() tea.Msg {
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		err := imapClient.DeleteMessage(uid)
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
	imapClient := a.imap // capture for nil check in closure

	return func() tea.Msg {
		if len(uids) == 0 {
			return bulkActionCompleteMsg{action: "moved to trash", count: 0}
		}
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		err := imapClient.MoveToTrashFromMailbox(uids, mailbox)
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
	imapClient := a.imap // capture for nil check in closure

	return func() tea.Msg {
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		err := imapClient.MoveToTrashFromMailbox([]imap.UID{uid}, mailbox)
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
	imapClient := a.imap // capture for nil check in closure

	return func() tea.Msg {
		if imapClient == nil {
			return draftSaveErrorMsg{err: fmt.Errorf("connecting to server")}
		}
		if err := imapClient.SaveDraft(to, subject, body); err != nil {
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

func (a *App) parseManualEvent(input string) tea.Cmd {
	client := a.aiClient
	prompt := ai.ParseCalendarEventPrompt(input, time.Now())
	provider := client.Provider()

	return func() tea.Msg {
		response, err := client.Call(prompt)
		if err != nil {
			return extractErrorMsg{err: err}
		}

		// Parse the event
		parsed, err := ai.ParseEventResponse(response)
		if err != nil {
			return extractErrorMsg{err: fmt.Errorf("failed to parse: %w", err)}
		}

		startTime, err := parsed.GetStartTime()
		if err != nil {
			return extractErrorMsg{err: fmt.Errorf("invalid start time: %w", err)}
		}

		endTime, err := parsed.GetEndTime()
		if err != nil {
			return extractErrorMsg{err: fmt.Errorf("invalid end time: %w", err)}
		}

		return extractResultMsg{
			found:     true,
			event:     parsed,
			startTime: startTime,
			endTime:   endTime,
			provider:  provider,
		}
	}
}

func (a *App) doExtractEvent(email *mail.Email) tea.Cmd {
	client := a.aiClient
	body := email.Body
	if body == "" {
		body = email.Snippet
	}
	prompt := ai.ExtractEventsPrompt(email.From, email.Subject, body, time.Now())
	provider := client.Provider()

	return func() tea.Msg {
		response, err := client.Call(prompt)
		if err != nil {
			return extractErrorMsg{err: err}
		}

		// Check if no events found
		if response == "NO_EVENTS_FOUND" || response == "" {
			return extractResultMsg{found: false, provider: provider}
		}

		// Parse the event
		parsed, err := ai.ParseEventResponse(response)
		if err != nil {
			return extractErrorMsg{err: fmt.Errorf("failed to parse event: %w", err)}
		}

		startTime, err := parsed.GetStartTime()
		if err != nil {
			return extractErrorMsg{err: fmt.Errorf("invalid start time: %w", err)}
		}

		endTime, err := parsed.GetEndTime()
		if err != nil {
			return extractErrorMsg{err: fmt.Errorf("invalid end time: %w", err)}
		}

		return extractResultMsg{
			found:     true,
			event:     parsed,
			startTime: startTime,
			endTime:   endTime,
			provider:  provider,
		}
	}
}

// loadCachedEmails loads emails from disk cache for instant display
func (a App) loadCachedEmails() tea.Cmd {
	account := a.currentAccount()
	accountEmail := ""
	if account != nil {
		accountEmail = account.Credentials.Email
	}
	if account == nil || a.diskCache == nil {
		return func() tea.Msg {
			return cachedEmailsLoadedMsg{emails: nil, accountEmail: accountEmail}
		}
	}

	mailbox := a.currentLabel

	return func() tea.Msg {
		cached, err := a.diskCache.LoadEmailsLimit(accountEmail, mailbox, 50)
		if err != nil || len(cached) == 0 {
			return cachedEmailsLoadedMsg{emails: nil, accountEmail: accountEmail}
		}

		// Convert cached emails to mail.Email format
		emails := make([]mail.Email, len(cached))
		for i, c := range cached {
			emails[i] = cachedToGmail(c)
		}
		return cachedEmailsLoadedMsg{emails: emails, accountEmail: accountEmail}
	}
}

// reloadFromCache reloads emails from disk cache (for manual refresh)
func (a App) reloadFromCache() tea.Cmd {
	account := a.currentAccount()
	if account == nil || a.diskCache == nil {
		return func() tea.Msg {
			return emailsLoadedMsg{emails: nil, accountEmail: ""}
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
		return emailsLoadedMsg{emails: emails, accountEmail: accountEmail}
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
			Encoding:    a.Encoding,
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
	case "new":
		// New email
		account := a.currentAccount()
		if account != nil {
			a.compose = NewComposeModel(account.Credentials.Email)
			a.compose.setSize(a.width, a.height)
			a.view = composeView
			return a, a.compose.Init()
		}

	case "reply":
		// Reply to selected email
		if email := a.mailList.SelectedEmail(); email != nil {
			account := a.currentAccount()
			if account != nil {
				a.compose = NewReplyModel(account.Credentials.Email, email)
				a.compose.setSize(a.width, a.height)
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
		// AI extract event from email
		if a.aiClient.Available() {
			if email := a.mailList.SelectedEmail(); email != nil {
				a.state = stateLoading
				a.statusMsg = "Extracting event with " + a.aiClient.Provider() + "..."
				return a, tea.Batch(a.spinner.Tick, a.doExtractEvent(email))
			}
		} else {
			a.statusMsg = "No AI CLI found (install claude, codex, gemini, vibe, or ollama)"
		}

	case "add":
		// Add calendar event (placeholder)
		a.statusMsg = "Add event: Not implemented yet"
	}

	return a, nil
}
