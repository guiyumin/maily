package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/emersion/go-imap/v2"
	"maily/internal/ai"
	"maily/internal/cache"
	"maily/internal/calendar"
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
	serverClient := a.serverClient
	imapClient := a.imap // fallback
	return func() tea.Msg {
		// Try server first
		if serverClient != nil {
			labels, err := serverClient.GetLabels(accountEmail)
			if err == nil {
				return labelsLoadedMsg{labels: labels, accountEmail: accountEmail}
			}
		}

		// Fall back to direct IMAP
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		labels, err := imapClient.ListMailboxes()
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
	serverClient := a.serverClient
	imapClient := a.imap // fallback
	return func() tea.Msg {
		// Try server first
		if serverClient != nil {
			cached, err := serverClient.Search(accountEmail, label, query)
			if err == nil {
				emails := make([]mail.Email, len(cached))
				for i, c := range cached {
					emails[i] = cachedToGmail(c)
				}
				return appSearchResultsMsg{emails: emails, query: query, accountEmail: accountEmail}
			}
		}

		// Fall back to direct IMAP
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		emails, err := imapClient.SearchMessages(label, query)
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
	serverClient := a.serverClient
	imapClient := a.imap // fallback

	return func() tea.Msg {
		if len(uids) == 0 {
			return bulkActionCompleteMsg{action: "marked as read", count: 0}
		}

		// Try server first
		if serverClient != nil {
			if err := serverClient.MarkMultiRead(accountEmail, mailbox, uids); err == nil {
				return bulkActionCompleteMsg{action: "marked as read", count: len(uids)}
			}
		}

		// Fall back to direct IMAP
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		if err := imapClient.MarkMessagesAsRead(uids); err != nil {
			return errorMsg{err: err, accountEmail: accountEmail}
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
	serverClient := a.serverClient
	imapClient := a.imap // fallback

	return func() tea.Msg {
		if len(uids) == 0 {
			return bulkActionCompleteMsg{action: "deleted", count: 0}
		}

		// Try server first
		if serverClient != nil {
			if err := serverClient.DeleteMulti(accountEmail, mailbox, uids); err == nil {
				return bulkActionCompleteMsg{action: "deleted", count: len(uids)}
			}
		}

		// Fall back to direct IMAP
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		err := imapClient.DeleteMessages(uids)
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to delete: %w", err), accountEmail: accountEmail}
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
	serverClient := a.serverClient
	imapClient := a.imap // fallback

	return func() tea.Msg {
		// Try server first
		if serverClient != nil {
			if err := serverClient.DeleteEmail(accountEmail, mailbox, uid); err == nil {
				return singleDeleteCompleteMsg{uid: uid}
			}
		}

		// Fall back to direct IMAP
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		err := imapClient.DeleteMessage(uid)
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to delete: %w", err), accountEmail: accountEmail}
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
	serverClient := a.serverClient
	imapClient := a.imap // fallback

	return func() tea.Msg {
		if len(uids) == 0 {
			return bulkActionCompleteMsg{action: "moved to trash", count: 0}
		}

		// Try server first
		if serverClient != nil {
			if err := serverClient.MoveMultiToTrash(accountEmail, mailbox, uids); err == nil {
				return bulkActionCompleteMsg{action: "moved to trash", count: len(uids)}
			}
		}

		// Fall back to direct IMAP
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		err := imapClient.MoveToTrashFromMailbox(uids, mailbox)
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to move to trash: %w", err), accountEmail: accountEmail}
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
	serverClient := a.serverClient
	imapClient := a.imap // fallback

	return func() tea.Msg {
		// Try server first
		if serverClient != nil {
			if err := serverClient.MoveToTrash(accountEmail, mailbox, uid); err == nil {
				return singleDeleteCompleteMsg{uid: uid}
			}
		}

		// Fall back to direct IMAP
		if imapClient == nil {
			return errorMsg{err: fmt.Errorf("connecting to server"), accountEmail: accountEmail}
		}
		err := imapClient.MoveToTrashFromMailbox([]imap.UID{uid}, mailbox)
		if err != nil {
			return errorMsg{err: fmt.Errorf("failed to move to trash: %w", err), accountEmail: accountEmail}
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

	// Convert compose attachments to mail attachments
	composeAttachments := a.compose.GetAttachments()
	var attachments []mail.AttachmentFile
	for _, att := range composeAttachments {
		attachments = append(attachments, mail.AttachmentFile{
			Path:        att.Path,
			Name:        att.Name,
			Size:        att.Size,
			ContentType: att.ContentType,
		})
	}

	return func() tea.Msg {
		smtpClient := mail.NewSMTPClient(&account.Credentials)

		var err error
		if len(attachments) > 0 {
			// Send with attachments
			if original != nil {
				err = smtpClient.ReplyWithAttachments(to, subject, body, original.MessageID, original.References, attachments)
			} else {
				err = smtpClient.SendWithAttachments(to, subject, body, attachments)
			}
		} else {
			// Send without attachments (original flow)
			if original != nil {
				err = smtpClient.Reply(to, subject, body, original.MessageID, original.References)
			} else {
				err = smtpClient.Send(to, subject, body)
			}
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
	body := email.BodyHTML
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

func (a *App) parseManualEvent(input string, email *mail.Email) tea.Cmd {
	client := a.aiClient

	// Include email context if available to resolve references like "them"
	var prompt string
	if email != nil {
		body := email.BodyHTML
		if body == "" {
			body = email.Snippet
		}
		prompt = ai.ParseCalendarEventWithContextPrompt(input, email.From, email.Subject, body, time.Now())
	} else {
		prompt = ai.ParseCalendarEventPrompt(input, time.Now())
	}
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
	body := email.BodyHTML
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

		// Check if no events found (case-insensitive, handle variations)
		responseLower := strings.ToLower(strings.TrimSpace(response))
		if response == "" || responseLower == "no_events_found" ||
			strings.Contains(responseLower, "no events found") ||
			strings.Contains(responseLower, "no event found") {
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

// addEventToCalendar creates a calendar event from the extracted event data
func (a *App) addEventToCalendar() tea.Cmd {
	event := a.extractedEvent
	startTime := a.extractedStart
	endTime := a.extractedEnd
	client := a.calClient

	return func() tea.Msg {
		if event == nil || client == nil {
			return calendarEventErrorMsg{err: fmt.Errorf("no event data or calendar unavailable")}
		}

		calEvent := calendar.Event{
			Title:              event.Title,
			StartTime:          startTime,
			EndTime:            endTime,
			Location:           event.Location,
			AlarmMinutesBefore: event.AlarmMinutesBefore,
		}

		eventID, err := client.CreateEvent(calEvent)
		if err != nil {
			return calendarEventErrorMsg{err: err}
		}

		return calendarEventCreatedMsg{eventID: eventID}
	}
}

// ReminderOptions defines available reminder choices (in minutes, 0 = none)
var ReminderOptions = []int{0, 5, 10, 15, 30, 60}

// ReminderLabels are the display labels for reminder options
var ReminderLabels = []string{"No reminder", "5 minutes", "10 minutes", "15 minutes", "30 minutes", "1 hour"}

// minutesToReminderIndex converts minutes to reminder option index
func minutesToReminderIndex(minutes int) int {
	for i, m := range ReminderOptions {
		if m == minutes {
			return i
		}
	}
	// Default to no reminder if not found
	return 0
}

// initExtractEditForm initializes the edit form with extracted event data
func (a *App) initExtractEditForm() {
	event := a.extractedEvent
	if event == nil {
		return
	}

	// Title
	a.extractEditTitle = textinput.New()
	a.extractEditTitle.Placeholder = "Event title"
	a.extractEditTitle.SetValue(event.Title)
	a.extractEditTitle.CharLimit = 100
	a.extractEditTitle.Width = 40
	a.extractEditTitle.Focus()

	// Date (YYYY-MM-DD)
	a.extractEditDate = textinput.New()
	a.extractEditDate.Placeholder = "YYYY-MM-DD"
	a.extractEditDate.SetValue(a.extractedStart.Format("2006-01-02"))
	a.extractEditDate.CharLimit = 10
	a.extractEditDate.Width = 12

	// Start time (HH:MM)
	a.extractEditStart = textinput.New()
	a.extractEditStart.Placeholder = "HH:MM"
	a.extractEditStart.SetValue(a.extractedStart.Format("15:04"))
	a.extractEditStart.CharLimit = 5
	a.extractEditStart.Width = 8

	// End time (HH:MM)
	a.extractEditEnd = textinput.New()
	a.extractEditEnd.Placeholder = "HH:MM"
	a.extractEditEnd.SetValue(a.extractedEnd.Format("15:04"))
	a.extractEditEnd.CharLimit = 5
	a.extractEditEnd.Width = 8

	// Location
	a.extractEditLocation = textinput.New()
	a.extractEditLocation.Placeholder = "Location (optional)"
	a.extractEditLocation.SetValue(event.Location)
	a.extractEditLocation.CharLimit = 100
	a.extractEditLocation.Width = 40

	// Reminder - convert minutes to index
	a.extractEditReminder = minutesToReminderIndex(event.AlarmMinutesBefore)

	a.extractEditFocus = 0
}

// updateExtractEditFocus updates focus state of edit form fields
func (a *App) updateExtractEditFocus() {
	a.extractEditTitle.Blur()
	a.extractEditDate.Blur()
	a.extractEditStart.Blur()
	a.extractEditEnd.Blur()
	a.extractEditLocation.Blur()

	switch a.extractEditFocus {
	case 0:
		a.extractEditTitle.Focus()
	case 1:
		a.extractEditDate.Focus()
	case 2:
		a.extractEditStart.Focus()
	case 3:
		a.extractEditEnd.Focus()
	case 4:
		a.extractEditLocation.Focus()
	}
}

// applyExtractEdits validates and applies edits to the extracted event
func (a *App) applyExtractEdits() error {
	// Parse date
	dateStr := a.extractEditDate.Value()
	date, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		return fmt.Errorf("invalid date format (use YYYY-MM-DD)")
	}

	// Parse start time
	startStr := a.extractEditStart.Value()
	startTime, err := time.Parse("15:04", startStr)
	if err != nil {
		return fmt.Errorf("invalid start time format (use HH:MM)")
	}

	// Parse end time
	endStr := a.extractEditEnd.Value()
	endTime, err := time.Parse("15:04", endStr)
	if err != nil {
		return fmt.Errorf("invalid end time format (use HH:MM)")
	}

	// Combine date and times
	a.extractedStart = time.Date(date.Year(), date.Month(), date.Day(),
		startTime.Hour(), startTime.Minute(), 0, 0, time.Local)
	a.extractedEnd = time.Date(date.Year(), date.Month(), date.Day(),
		endTime.Hour(), endTime.Minute(), 0, 0, time.Local)

	// Validate end > start
	if !a.extractedEnd.After(a.extractedStart) {
		return fmt.Errorf("end time must be after start time")
	}

	// Update event fields
	a.extractedEvent.Title = a.extractEditTitle.Value()
	a.extractedEvent.Location = a.extractEditLocation.Value()
	a.extractedEvent.AlarmMinutesBefore = ReminderOptions[a.extractEditReminder]

	return nil
}

// loadCachedEmails loads emails from server (or falls back to disk cache)
func (a App) loadCachedEmails() tea.Cmd {
	account := a.currentAccount()
	accountEmail := ""
	if account != nil {
		accountEmail = account.Credentials.Email
	}
	if account == nil {
		return func() tea.Msg {
			return cachedEmailsLoadedMsg{emails: nil, accountEmail: accountEmail}
		}
	}

	mailbox := a.currentLabel
	limit := a.cfg.MaxEmails
	serverClient := a.serverClient
	diskCache := a.diskCache

	return func() tea.Msg {
		// Try server first
		if serverClient != nil {
			cached, err := serverClient.GetEmails(accountEmail, mailbox, limit)
			if err == nil && len(cached) > 0 {
				emails := make([]mail.Email, len(cached))
				for i, c := range cached {
					emails[i] = cachedToGmail(c)
				}
				return cachedEmailsLoadedMsg{emails: emails, accountEmail: accountEmail}
			}
		}

		// Fall back to disk cache
		if diskCache != nil {
			cached, err := diskCache.LoadEmailsLimit(accountEmail, mailbox, limit)
			if err == nil && len(cached) > 0 {
				emails := make([]mail.Email, len(cached))
				for i, c := range cached {
					emails[i] = cachedToGmail(c)
				}
				return cachedEmailsLoadedMsg{emails: emails, accountEmail: accountEmail}
			}
		}

		return cachedEmailsLoadedMsg{emails: nil, accountEmail: accountEmail}
	}
}

// reloadFromCache reloads emails from server (or disk cache for fallback)
func (a App) reloadFromCache() tea.Cmd {
	account := a.currentAccount()
	if account == nil {
		return func() tea.Msg {
			return emailsLoadedMsg{emails: nil, accountEmail: ""}
		}
	}

	accountEmail := account.Credentials.Email
	mailbox := a.currentLabel
	limit := int(a.emailLimit)
	serverClient := a.serverClient
	diskCache := a.diskCache

	return func() tea.Msg {
		// Try server first
		if serverClient != nil {
			cached, err := serverClient.GetEmails(accountEmail, mailbox, limit)
			if err == nil {
				emails := make([]mail.Email, len(cached))
				for i, c := range cached {
					emails[i] = cachedToGmail(c)
				}
				return emailsLoadedMsg{emails: emails, accountEmail: accountEmail}
			}
		}

		// Fall back to disk cache
		if diskCache != nil {
			cached, err := diskCache.LoadEmailsLimit(accountEmail, mailbox, limit)
			if err == nil {
				emails := make([]mail.Email, len(cached))
				for i, c := range cached {
					emails[i] = cachedToGmail(c)
				}
				return emailsLoadedMsg{emails: emails, accountEmail: accountEmail}
			}
		}

		return emailsLoadedMsg{emails: nil, accountEmail: accountEmail}
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
		BodyHTML:     c.BodyHTML,
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
