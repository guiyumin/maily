package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/emersion/go-imap/v2"

	"maily/config"
	"maily/internal/ai"
	"maily/internal/auth"
	"maily/internal/cache"
	"maily/internal/calendar"
	"maily/internal/mail"
	"maily/internal/ui/components"
)

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
	store           *auth.AccountStore
	cfg             *config.Config
	accountIdx      int
	imap            *mail.IMAPClient
	imapCache       map[int]*mail.IMAPClient
	emailCache      map[string][]mail.Email // key: "accountIdx:label"
	diskCache       *cache.Cache            // persistent disk cache
	mailList        components.MailList
	viewport        viewport.Model
	spinner         spinner.Model
	state           state
	view            view
	width           int
	height          int
	err             error
	errAccountEmail string // which account the error belongs to
	statusMsg       string
	confirmDelete   bool
	deleteOption    components.DeleteOption // selected option in delete dialog
	emailLimit      uint32

	// Labels
	labelPicker     components.LabelPicker
	currentLabel    string // current mailbox/label being viewed
	showLabelPicker bool   // showing label picker view

	// Search
	searchInput    textinput.Model
	searchMode     bool // typing search query
	isSearchResult bool // showing search results
	searchQuery    string
	inboxCache     []mail.Email

	// Multi-select (search mode only)
	selected map[imap.UID]bool

	// Scroll throttling (count-based)
	scrollCount int

	// Reply/Compose
	compose ComposeModel

	// Command palette
	commandPalette     components.CommandPalette
	showCommandPalette bool

	// AI
	aiClient      *ai.Client
	showSummary   bool
	summaryText   string
	summarySource string // which AI provider was used

	// Extract
	showExtract       bool
	extractedEvent    *ai.ParsedEvent
	extractedStart    time.Time
	extractedEnd      time.Time
	extractedProvider string // which AI provider was used

	// Extract edit form
	showExtractEdit      bool
	extractEditTitle     textinput.Model
	extractEditDate      textinput.Model
	extractEditStart     textinput.Model
	extractEditEnd       textinput.Model
	extractEditLocation  textinput.Model
	extractEditReminder  int // index into reminderOptions: 0=none, 1=5min, 2=10min, 3=15min, 4=30min, 5=1hr
	extractEditFocus     int // 0=title, 1=date, 2=start, 3=end, 4=location, 5=reminder, 6=save, 7=cancel

	// Calendar
	calClient calendar.Client

	// Manual extract input (when no event found)
	showExtractInput bool
	extractInput     textinput.Model

	// Attachments (download)
	showAttachmentPicker bool
	attachmentIdx        int

	// File picker (for compose attachments)
	showFilePicker bool
	filePicker     components.FilePicker
}

type emailsLoadedMsg struct {
	emails       []mail.Email
	accountEmail string // which account this belongs to
	uidValidity  uint32 // for cache consistency with daemon
}

type errorMsg struct {
	err          error
	accountEmail string // which account this error belongs to
}

type clientReadyMsg struct {
	imap         *mail.IMAPClient
	accountEmail string // which account this belongs to
}

type appSearchResultsMsg struct {
	emails       []mail.Email
	query        string
	accountEmail string // which account this belongs to
}

type labelsLoadedMsg struct {
	labels       []string
	accountEmail string // which account this belongs to
}

type replySentMsg struct{}

type replySendErrorMsg struct {
	err error
}

type summaryResultMsg struct {
	summary  string
	provider string
}

type summaryErrorMsg struct {
	err error
}

type extractResultMsg struct {
	found     bool
	event     *ai.ParsedEvent
	startTime time.Time
	endTime   time.Time
	provider  string
}

type extractErrorMsg struct {
	err error
}

type calendarEventCreatedMsg struct {
	eventID string
}

type calendarEventErrorMsg struct {
	err error
}

type cachedEmailsLoadedMsg struct {
	emails       []mail.Email
	accountEmail string // which account this belongs to
}

type singleDeleteCompleteMsg struct {
	uid imap.UID
}

type autoRefreshTickMsg struct{}

type attachmentDownloadedMsg struct {
	filename string
	path     string
}

type attachmentDownloadErrorMsg struct {
	err error
}

const autoRefreshInterval = 5 * time.Minute

func scheduleAutoRefresh() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(t time.Time) tea.Msg {
		return autoRefreshTickMsg{}
	})
}

func NewApp(store *auth.AccountStore, cfg *config.Config) App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = components.SpinnerStyle

	si := textinput.New()
	si.Placeholder = "Search emails..."
	si.CharLimit = 200
	si.Width = 40

	vp := viewport.New(80, 24) // Default size, will be resized by WindowSizeMsg
	vp.Style = lipgloss.NewStyle().Padding(1, 4, 3, 4)

	// Initialize disk cache (ignore error, will just skip cache)
	diskCache, _ := cache.New()

	// Initialize calendar client (ignore error, will just skip calendar features)
	calClient, _ := calendar.NewClient()

	return App{
		store:          store,
		cfg:            cfg,
		accountIdx:     0,
		imapCache:      make(map[int]*mail.IMAPClient),
		emailCache:     make(map[string][]mail.Email),
		diskCache:      diskCache,
		mailList:       components.NewMailList(),
		viewport:       vp,
		spinner:        s,
		state:          stateLoading,
		view:           listView,
		emailLimit:     uint32(cfg.MaxEmails),
		labelPicker:    components.NewLabelPicker(),
		currentLabel:   "INBOX",
		searchInput:    si,
		selected:       make(map[imap.UID]bool),
		commandPalette: components.NewCommandPalette(),
		aiClient:       ai.NewClient(),
		calClient:      calClient,
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
		a.loadCachedEmails(),
		scheduleAutoRefresh(),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle command palette input
		if a.showCommandPalette {
			switch msg.String() {
			case "esc":
				a.showCommandPalette = false
				return a, nil
			default:
				var cmd tea.Cmd
				a.commandPalette, cmd = a.commandPalette.Update(msg)
				return a, cmd
			}
		}

		// Handle file picker input (for compose attachments)
		if a.showFilePicker {
			var cmd tea.Cmd
			a.filePicker, cmd = a.filePicker.Update(msg)
			return a, cmd
		}

		// Handle compose view input
		if a.view == composeView {
			var cmd tea.Cmd
			a.compose, cmd = a.compose.Update(msg)
			return a, cmd
		}

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

		// Handle extract input mode
		if a.showExtractInput {
			switch msg.String() {
			case "esc":
				a.showExtractInput = false
				a.extractInput.Blur()
				a.extractInput.SetValue("")
			case "enter":
				input := a.extractInput.Value()
				if input != "" {
					a.showExtractInput = false
					a.extractInput.Blur()
					a.state = stateLoading
					a.statusMsg = "Parsing with " + a.aiClient.Provider() + "..."
					return a, tea.Batch(a.spinner.Tick, a.parseManualEvent(input))
				}
			default:
				var cmd tea.Cmd
				a.extractInput, cmd = a.extractInput.Update(msg)
				return a, cmd
			}
			return a, nil
		}

		// Handle label picker navigation
		if a.showLabelPicker {
			switch msg.String() {
			case "up", "down", "k", "j":
				var cmd tea.Cmd
				a.labelPicker, cmd = a.labelPicker.Update(msg)
				return a, cmd
			case "enter":
				// Select label and load emails
				newLabel := a.labelPicker.CursorLabel()
				a.showLabelPicker = false
				if newLabel != a.currentLabel {
					a.currentLabel = newLabel
					a.labelPicker.SetSelected(newLabel)
					a.state = stateLoading
					a.statusMsg = "Loading..."
					return a, tea.Batch(a.spinner.Tick, a.loadEmails())
				}
				return a, nil
			case "esc", "g":
				a.showLabelPicker = false
				return a, nil
			case "q":
				return a, tea.Quit
			}
			return a, nil
		}

		// Handle attachment picker navigation
		if a.showAttachmentPicker {
			email := a.mailList.SelectedEmail()
			if email == nil {
				a.showAttachmentPicker = false
				return a, nil
			}
			switch msg.String() {
			case "tab":
				// Cycle through: 0 (Download All) -> 1..N (individual files) -> 0
				totalItems := len(email.Attachments) + 1 // +1 for "Download All"
				a.attachmentIdx = (a.attachmentIdx + 1) % totalItems
				return a, nil
			case "shift+tab":
				// Cycle backwards
				totalItems := len(email.Attachments) + 1
				a.attachmentIdx = (a.attachmentIdx - 1 + totalItems) % totalItems
				return a, nil
			case "q":
				return a, tea.Quit
			}
		}

		// Handle extract edit form
		if a.showExtractEdit {
			switch msg.String() {
			case "esc":
				// Go back to extract preview without saving
				a.showExtractEdit = false
				return a, nil
			case "tab":
				a.extractEditFocus = (a.extractEditFocus + 1) % 8
				a.updateExtractEditFocus()
				return a, nil
			case "shift+tab":
				a.extractEditFocus = (a.extractEditFocus - 1 + 8) % 8
				a.updateExtractEditFocus()
				return a, nil
			case "up", "k":
				// For reminder field, cycle through options
				if a.extractEditFocus == 5 {
					a.extractEditReminder = (a.extractEditReminder - 1 + 6) % 6
					return a, nil
				}
			case "down", "j":
				// For reminder field, cycle through options
				if a.extractEditFocus == 5 {
					a.extractEditReminder = (a.extractEditReminder + 1) % 6
					return a, nil
				}
			case "enter":
				switch a.extractEditFocus {
				case 6: // Save button
					if err := a.applyExtractEdits(); err != nil {
						a.statusMsg = fmt.Sprintf("Invalid input: %v", err)
						return a, nil
					}
					a.showExtractEdit = false
					a.statusMsg = "Changes saved"
					return a, nil
				case 7: // Cancel button
					a.showExtractEdit = false
					return a, nil
				default:
					// Move to next field
					a.extractEditFocus++
					a.updateExtractEditFocus()
					return a, nil
				}
			default:
				// Route input to focused field
				var cmd tea.Cmd
				switch a.extractEditFocus {
				case 0:
					a.extractEditTitle, cmd = a.extractEditTitle.Update(msg)
				case 1:
					a.extractEditDate, cmd = a.extractEditDate.Update(msg)
				case 2:
					a.extractEditStart, cmd = a.extractEditStart.Update(msg)
				case 3:
					a.extractEditEnd, cmd = a.extractEditEnd.Update(msg)
				case 4:
					a.extractEditLocation, cmd = a.extractEditLocation.Update(msg)
				// case 5: reminder uses up/down, no text input
				// case 6, 7: buttons, no text input
				}
				return a, cmd
			}
		}

		// Handle extract dialog (add to calendar)
		if a.showExtract && a.extractedEvent != nil {
			switch msg.String() {
			case "enter":
				// Add event to calendar
				if a.calClient == nil {
					a.statusMsg = "Calendar not available"
					a.showExtract = false
					a.extractedEvent = nil
					return a, nil
				}
				a.state = stateLoading
				a.statusMsg = "Adding to calendar..."
				return a, tea.Batch(a.spinner.Tick, a.addEventToCalendar())
			case "e":
				// Enter edit mode
				a.initExtractEditForm()
				a.showExtractEdit = true
				return a, textinput.Blink
			case "esc":
				a.showExtract = false
				a.extractedEvent = nil
				a.extractedProvider = ""
				return a, nil
			}
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
		case "f":
			// Show label picker (when not in search/confirm mode)
			if a.state == stateReady && !a.confirmDelete && !a.searchMode && !a.isSearchResult && a.view == listView {
				a.labelPicker.SetSelected(a.currentLabel)
				a.showLabelPicker = true
				return a, nil
			}
		case "esc":
			if a.showAttachmentPicker {
				a.showAttachmentPicker = false
				return a, nil
			} else if a.showSummary {
				a.showSummary = false
				a.summaryText = ""
				a.summarySource = ""
			} else if a.confirmDelete {
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
			// Open command palette
			if a.state == stateReady && !a.confirmDelete && !a.searchMode {
				viewName := "list"
				if a.view == readView {
					viewName = "read"
				}
				a.commandPalette.SetView(viewName)
				a.commandPalette.SetSize(a.width, a.height)
				a.showCommandPalette = true
				return a, a.commandPalette.Init()
			}
		case "enter":
			// Handle attachment picker
			if a.showAttachmentPicker {
				email := a.mailList.SelectedEmail()
				if email != nil {
					a.showAttachmentPicker = false
					a.state = stateLoading
					if a.attachmentIdx == 0 {
						// Download All
						a.statusMsg = fmt.Sprintf("Downloading %d attachments...", len(email.Attachments))
						return a, tea.Batch(a.spinner.Tick, a.downloadAllAttachments(email))
					} else {
						// Individual attachment (index shifted by 1)
						attIdx := a.attachmentIdx - 1
						if attIdx < len(email.Attachments) {
							a.statusMsg = "Downloading " + email.Attachments[attIdx].Filename + "..."
							return a, tea.Batch(a.spinner.Tick, a.downloadAttachment(email, attIdx))
						}
					}
				}
			}
			// Handle delete confirmation dialog
			if a.confirmDelete {
				switch a.deleteOption {
				case components.DeleteOptionTrash:
					// Move to trash
					if a.isSearchResult && a.selectedCount() > 0 {
						a.state = stateLoading
						a.statusMsg = "Moving to trash..."
						a.confirmDelete = false
						return a, tea.Batch(a.spinner.Tick, a.moveSelectedToTrash())
					} else if email := a.mailList.SelectedEmail(); email != nil {
						uid := email.UID
						a.view = listView
						a.state = stateLoading
						a.statusMsg = "Moving to trash..."
						a.confirmDelete = false
						return a, tea.Batch(a.spinner.Tick, a.moveSingleToTrash(uid))
					}
				case components.DeleteOptionPermanent:
					// Permanent delete
					if a.isSearchResult && a.selectedCount() > 0 {
						a.state = stateLoading
						a.statusMsg = "Deleting permanently..."
						a.confirmDelete = false
						return a, tea.Batch(a.spinner.Tick, a.deleteSelectedEmails())
					} else if email := a.mailList.SelectedEmail(); email != nil {
						uid := email.UID
						a.view = listView
						a.state = stateLoading
						a.statusMsg = "Deleting permanently..."
						a.confirmDelete = false
						return a, tea.Batch(a.spinner.Tick, a.deleteSingleEmail(uid))
					}
				case components.DeleteOptionCancel:
					a.confirmDelete = false
					a.statusMsg = ""
				}
				return a, nil
			}
			// Normal enter - open email
			if a.view == listView && a.state == stateReady {
				if email := a.mailList.SelectedEmail(); email != nil {
					a.view = readView
					a.viewport.SetContent(a.renderEmailContent(*email))
					a.viewport.GotoTop()

					if email.Unread {
						uid := email.UID
						account := a.currentAccount()
						imapClient := a.imap
						label := a.currentLabel
						// Update in-memory state immediately for responsive UI
						a.mailList.MarkAsRead(uid)
						go func() {
							if imapClient != nil {
								imapClient.MarkAsRead(uid)
							}
							// Update disk cache
							if a.diskCache != nil && account != nil {
								a.diskCache.UpdateEmailFlags(account.Credentials.Email, label, uid, false)
							}
						}()
					}
				}
			}
		case "n":
			// new email
			if a.state == stateReady && !a.confirmDelete && a.view == listView {
				account := a.currentAccount()
				if account != nil {
					a.compose = NewComposeModel(account.Credentials.Email)
					a.compose.setSize(a.width, a.height)
					a.view = composeView
					return a, a.compose.Init()
				}
			}
		case "r":
			// Reply to email (in list or read view)
			if a.state == stateReady && !a.confirmDelete && (a.view == listView || a.view == readView) {
				if email := a.mailList.SelectedEmail(); email != nil {
					account := a.currentAccount()
					if account != nil {
						a.compose = NewReplyModel(account.Credentials.Email, email)
						a.compose.setSize(a.width, a.height)
						a.view = composeView
						return a, a.compose.Init()
					}
				}
			}
		case "R":
			// Shift+R for refresh from IMAP server
			if a.state == stateReady && !a.isSearchResult && a.view == listView {
				a.state = stateLoading
				a.statusMsg = "Refreshing..."
				return a, tea.Batch(a.spinner.Tick, a.loadEmails())
			}
		case "s":
			// Context-aware: search in list view, summarize in read view
			if a.state == stateReady && !a.confirmDelete && !a.showSummary {
				if a.view == listView && !a.isSearchResult {
					// Search mode
					a.searchMode = true
					a.searchInput.Focus()
					return a, textinput.Blink
				} else if a.view == readView {
					// Summarize with AI
					if !a.aiClient.Available() {
						a.statusMsg = "No AI CLI found (install claude, codex, gemini, vibe, or ollama)"
						return a, nil
					}
					email := a.mailList.SelectedEmail()
					if email != nil {
						a.state = stateLoading
						a.statusMsg = "Summarizing with " + a.aiClient.Provider() + "..."
						return a, tea.Batch(a.spinner.Tick, a.summarizeEmail(email))
					}
				}
			}
		case "e":
			// Extract event from email (read view only)
			if a.state == stateReady && a.view == readView && !a.confirmDelete && !a.showExtract {
				if !a.aiClient.Available() {
					a.statusMsg = "No AI CLI found (install claude, codex, gemini, vibe, or ollama)"
					return a, nil
				}
				email := a.mailList.SelectedEmail()
				if email != nil {
					a.state = stateLoading
					a.statusMsg = "Extracting event with " + a.aiClient.Provider() + "..."
					return a, tea.Batch(a.spinner.Tick, a.doExtractEvent(email))
				}
			}
		case "a":
			// Download attachments (read view only)
			if a.state == stateReady && a.view == readView && !a.confirmDelete && !a.showAttachmentPicker {
				email := a.mailList.SelectedEmail()
				if email == nil || len(email.Attachments) == 0 {
					a.statusMsg = "No attachments"
					return a, nil
				}
				// Always show picker
				a.showAttachmentPicker = true
				a.attachmentIdx = 0
				return a, nil
			}
				// Select/deselect all (search mode only)
			if a.isSearchResult && a.view == listView && a.state == stateReady {
				emails := a.mailList.Emails()
				allSelected := len(a.selected) == len(emails) && len(emails) > 0
				for _, email := range emails {
					if !a.selected[email.UID] {
						allSelected = false
						break
					}
				}
				if allSelected {
					a.selected = make(map[imap.UID]bool)
				} else {
					for _, email := range emails {
						a.selected[email.UID] = true
					}
				}
				a.mailList.SetSelections(a.selected)
			}
		case "d":
			if a.state == stateReady && !a.confirmDelete {
				// In search mode with selections, delete selected emails
				if a.isSearchResult && a.selectedCount() > 0 {
					a.confirmDelete = true
					a.deleteOption = components.DeleteOptionTrash // default to Trash
				} else if a.mailList.SelectedEmail() != nil {
					a.confirmDelete = true
					a.deleteOption = components.DeleteOptionTrash // default to Trash
				}
			}
		case "left", "h":
			if a.confirmDelete {
				if a.deleteOption > 0 {
					a.deleteOption--
				}
			}
		case "right":
			if a.confirmDelete {
				if a.deleteOption < components.DeleteOptionCancel {
					a.deleteOption++
				}
			}
		case "l":
			if a.view == listView && a.state == stateReady && !a.confirmDelete && !a.isSearchResult {
				a.emailLimit += uint32(a.cfg.MaxEmails)
				a.state = stateLoading
				a.statusMsg = fmt.Sprintf("Loading %d emails...", a.emailLimit)
				return a, tea.Batch(a.spinner.Tick, a.reloadFromCache())
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
		case "m": // Mark read/unread (search mode only, for selected emails)
			if a.isSearchResult && a.view == listView && a.state == stateReady && a.selectedCount() > 0 {
				a.state = stateLoading
				a.statusMsg = "Marking as read..."
				return a, tea.Batch(a.spinner.Tick, a.markSelectedAsRead())
			}
		case "tab":
			if len(a.store.Accounts) > 1 && !a.confirmDelete && !a.isSearchResult && !a.showLabelPicker {
				// Save current emails to cache (only if not in error state)
				if a.state != stateError {
					if emails := a.mailList.Emails(); len(emails) > 0 {
						cacheKey := fmt.Sprintf("%d:%s", a.accountIdx, a.currentLabel)
						a.emailCache[cacheKey] = emails
					}
					// Save current IMAP connection to cache
					if a.imap != nil {
						a.imapCache[a.accountIdx] = a.imap
					}
				}

				// Switch to next account
				a.accountIdx = (a.accountIdx + 1) % len(a.store.Accounts)
				a.view = listView
				a.currentLabel = "INBOX" // Reset to inbox on account switch
				a.showLabelPicker = false
				// Clear error state from previous account
				a.err = nil

				// Check if we have in-memory cached data for this account's inbox
				cacheKey := fmt.Sprintf("%d:%s", a.accountIdx, a.currentLabel)
				if cached, ok := a.emailCache[cacheKey]; ok && len(cached) > 0 {
					a.imap = a.imapCache[a.accountIdx]
					a.mailList.SetEmails(cached)
					a.state = stateReady
					labelName := components.GetLabelDisplayName(a.currentLabel)
					a.statusMsg = fmt.Sprintf("%s: %d emails", labelName, len(cached))
					return a, nil
				}

				// Try disk cache first, then init IMAP after cache load
				a.imap = a.imapCache[a.accountIdx]
				a.state = stateLoading
				a.emailLimit = uint32(a.cfg.MaxEmails)
				a.mailList.SetEmails(nil)
				a.statusMsg = "Loading..."
				return a, tea.Batch(a.spinner.Tick, a.loadCachedEmails())
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
		a.mailList.SetSize(msg.Width, msg.Height-7) // account for 2-row status bar
		a.labelPicker.SetSize(msg.Width, msg.Height)
		a.viewport.Width = msg.Width - 8
		a.viewport.Height = msg.Height - 8
		// Update compose model size (Update is called at end of function)
		if a.view == composeView {
			a.compose.setSize(msg.Width, msg.Height)
		}
		// Update file picker size if visible
		if a.showFilePicker {
			a.filePicker.SetSize(msg.Width, msg.Height)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case clientReadyMsg:
		// Ignore messages from other accounts (stale messages after switching)
		currentAccount := a.currentAccount()
		currentEmail := ""
		if currentAccount != nil {
			currentEmail = currentAccount.Credentials.Email
		}
		if msg.accountEmail != "" && msg.accountEmail != currentEmail {
			return a, nil
		}
		a.imap = msg.imap
		a.imapCache[a.accountIdx] = msg.imap
		// If we have emails visible, load labels silently (user already has usable UI)
		if len(a.mailList.Emails()) > 0 {
			return a, a.loadLabels()
		}
		a.statusMsg = "Loading labels..."
		return a, a.loadLabels()

	case labelsLoadedMsg:
		// Ignore messages from other accounts (stale messages after switching)
		currentAccount := a.currentAccount()
		currentEmail := ""
		if currentAccount != nil {
			currentEmail = currentAccount.Credentials.Email
		}
		if msg.accountEmail != "" && msg.accountEmail != currentEmail {
			return a, nil
		}
		a.labelPicker.SetLabels(msg.labels)
		// Skip server fetch if we have cached emails and cache is fresh (synced within 5 minutes)
		if len(a.mailList.Emails()) > 0 && a.diskCache != nil {
			if a.diskCache.IsFresh(currentEmail, a.currentLabel, 5*time.Minute) {
				// Cache is fresh, no need to fetch from server
				a.state = stateReady
				labelName := components.GetLabelDisplayName(a.currentLabel)
				a.statusMsg = fmt.Sprintf("%s: %d emails", labelName, len(a.mailList.Emails()))
				return a, nil
			}
		}
		// Cache is stale or empty - fetch from server
		// If we have cached emails, UI is already usable, fetch silently
		if len(a.mailList.Emails()) > 0 {
			// Don't show "Loading..." - UI is already usable
			return a, a.loadEmails()
		}
		a.statusMsg = "Loading emails..."
		return a, a.loadEmails()

	case cachedEmailsLoadedMsg:
		// Ignore messages from other accounts (stale messages after switching)
		currentAccount := a.currentAccount()
		currentEmail := ""
		if currentAccount != nil {
			currentEmail = currentAccount.Credentials.Email
		}
		if msg.accountEmail != "" && msg.accountEmail != currentEmail {
			return a, nil
		}
		// Only use cached emails if we haven't loaded from server yet
		if len(msg.emails) > 0 && len(a.mailList.Emails()) == 0 {
			a.mailList.SetEmails(msg.emails)
			labelName := components.GetLabelDisplayName(a.currentLabel)
			// Check if cache is fresh - if so, we're done (no need to fetch from server)
			if a.diskCache != nil && a.diskCache.IsFresh(currentEmail, a.currentLabel, 5*time.Minute) {
				a.state = stateReady
				a.statusMsg = fmt.Sprintf("%s: %d emails", labelName, len(msg.emails))
			} else {
				// Cache is stale, show emails but keep loading state for background fetch
				a.state = stateReady
				a.statusMsg = fmt.Sprintf("%s: %d emails", labelName, len(msg.emails))
			}
		}
		return a, a.initClient()

	case emailsLoadedMsg:
		// Ignore messages from other accounts (stale messages after switching)
		currentAccount := a.currentAccount()
		currentEmail := ""
		if currentAccount != nil {
			currentEmail = currentAccount.Credentials.Email
		}
		if msg.accountEmail != "" && msg.accountEmail != currentEmail {
			return a, nil
		}
		a.mailList.SetEmails(msg.emails)
		cacheKey := fmt.Sprintf("%d:%s", a.accountIdx, a.currentLabel)
		a.emailCache[cacheKey] = msg.emails
		a.state = stateReady
		labelName := components.GetLabelDisplayName(a.currentLabel)
		a.statusMsg = fmt.Sprintf("%s: %d emails", labelName, len(msg.emails))
		// Update cache metadata so future runs know cache is fresh
		if a.diskCache != nil && currentEmail != "" && msg.uidValidity != 0 {
			uidValidity := msg.uidValidity
			label := a.currentLabel
			go func() {
				meta := &cache.Metadata{UIDValidity: uidValidity, LastSync: time.Now()}
				a.diskCache.SaveMetadata(currentEmail, label, meta)
			}()
		}

	case autoRefreshTickMsg:
		// Schedule next tick
		cmds = append(cmds, scheduleAutoRefresh())
		// Only refresh if in list view, ready state, and not in any dialog
		if a.view == listView && a.state == stateReady && !a.confirmDelete && !a.searchMode && !a.showLabelPicker && !a.showCommandPalette && !a.isSearchResult {
			a.state = stateLoading
			a.statusMsg = "Auto-refreshing..."
			cmds = append(cmds, a.spinner.Tick, a.loadEmails())
		}
		return a, tea.Batch(cmds...)

	case errorMsg:
		// Ignore errors from other accounts (stale errors after switching)
		currentAccount := a.currentAccount()
		currentEmail := ""
		if currentAccount != nil {
			currentEmail = currentAccount.Credentials.Email
		}
		if msg.accountEmail != "" && msg.accountEmail != currentEmail {
			return a, nil
		}
		a.state = stateError
		a.err = msg.err
		a.errAccountEmail = msg.accountEmail

	case appSearchResultsMsg:
		// Ignore messages from other accounts (stale messages after switching)
		currentAccount := a.currentAccount()
		currentEmail := ""
		if currentAccount != nil {
			currentEmail = currentAccount.Credentials.Email
		}
		if msg.accountEmail != "" && msg.accountEmail != currentEmail {
			return a, nil
		}
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

	case singleDeleteCompleteMsg:
		a.state = stateReady
		a.mailList.RemoveByUID(msg.uid)
		a.statusMsg = "Successfully deleted 1 email"

	case replySentMsg:
		a.state = stateReady
		a.view = listView
		a.statusMsg = "Reply sent!"

	case replySendErrorMsg:
		a.state = stateReady
		a.view = composeView
		a.statusMsg = fmt.Sprintf("Send failed: %v", msg.err)

	case components.CommandSelectedMsg:
		a.showCommandPalette = false
		return a.executeCommand(msg.Command)

	case SendMsg:
		// Send button pressed in compose view
		a.state = stateLoading
		a.statusMsg = "Sending..."
		return a, tea.Batch(a.spinner.Tick, a.sendReply())

	case SaveDraftMsg:
		// Save Draft button pressed
		a.state = stateLoading
		a.statusMsg = "Saving draft..."
		return a, tea.Batch(a.spinner.Tick, a.saveDraft())

	case draftSavedMsg:
		a.state = stateReady
		if a.compose.isReply {
			a.view = readView
		} else {
			a.view = listView
		}
		a.statusMsg = "Draft saved!"

	case draftSaveErrorMsg:
		a.state = stateReady
		a.statusMsg = fmt.Sprintf("Failed to save draft: %v", msg.err)

	case CancelMsg:
		// Cancel button pressed in compose view
		if a.compose.isReply {
			a.view = readView
		} else {
			a.view = listView
		}
		a.statusMsg = "Cancelled"

	case OpenFilePickerMsg:
		// Open file picker from compose view
		a.filePicker = components.NewFilePicker()
		a.filePicker.SetSize(a.width, a.height)
		a.showFilePicker = true
		return a, nil

	case components.FileSelectedMsg:
		// File selected from file picker
		a.showFilePicker = false
		contentType := ""
		if err := a.compose.AddAttachment(msg.Path, msg.Name, contentType, msg.Size); err != nil {
			a.statusMsg = fmt.Sprintf("Cannot attach: %v", err)
		} else {
			a.statusMsg = fmt.Sprintf("Attached: %s", msg.Name)
		}
		return a, nil

	case components.FilePickerCancelledMsg:
		// File picker cancelled
		a.showFilePicker = false
		return a, nil

	case summaryResultMsg:
		a.state = stateReady
		a.showSummary = true
		a.summaryText = msg.summary
		a.summarySource = msg.provider
		a.statusMsg = ""

	case summaryErrorMsg:
		a.state = stateReady
		a.statusMsg = fmt.Sprintf("Summarize failed: %v", msg.err)

	case extractResultMsg:
		a.state = stateReady
		if !msg.found {
			// No event found - prompt user to type event details
			a.extractInput = textinput.New()
			a.extractInput.Placeholder = "e.g., tomorrow 2pm meeting with John"
			a.extractInput.Focus()
			a.extractInput.CharLimit = 200
			a.extractInput.Width = 50
			a.showExtractInput = true
			a.extractedProvider = msg.provider
			a.statusMsg = ""
			return a, textinput.Blink
		} else {
			a.showExtract = true
			a.extractedEvent = msg.event
			a.extractedStart = msg.startTime
			a.extractedEnd = msg.endTime
			a.extractedProvider = msg.provider
			a.statusMsg = ""
		}

	case extractErrorMsg:
		a.state = stateReady
		a.statusMsg = fmt.Sprintf("Extract failed: %v", msg.err)

	case calendarEventCreatedMsg:
		a.state = stateReady
		a.showExtract = false
		a.showExtractEdit = false
		a.extractedEvent = nil
		a.extractedProvider = ""
		a.statusMsg = "Event added to calendar"

	case calendarEventErrorMsg:
		a.state = stateReady
		a.showExtract = false
		a.showExtractEdit = false
		a.extractedEvent = nil
		a.statusMsg = fmt.Sprintf("Failed to add event: %v", msg.err)

	case attachmentDownloadedMsg:
		a.state = stateReady
		a.statusMsg = fmt.Sprintf("Downloaded %s to ~/Downloads/maily", msg.filename)

	case attachmentDownloadErrorMsg:
		a.state = stateReady
		a.statusMsg = fmt.Sprintf("Download failed: %v", msg.err)
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

	if a.view == composeView {
		var cmd tea.Cmd
		a.compose, cmd = a.compose.Update(msg)
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
		canSwitch := len(a.store.Accounts) > 1
		content = components.RenderError(a.width, a.height, a.err, a.errAccountEmail, canSwitch)
	case stateReady:
		switch a.view {
		case listView:
			content = components.RenderListView(a.width, a.height, a.mailList.View())
		case readView:
			if email := a.mailList.SelectedEmail(); email != nil {
				var attachments []components.AttachmentInfo
				for _, att := range email.Attachments {
					attachments = append(attachments, components.AttachmentInfo{
						Filename:    att.Filename,
						ContentType: att.ContentType,
						Size:        att.Size,
					})
				}
				emailData := components.EmailViewData{
					From:        email.From,
					To:          email.To,
					Subject:     email.Subject,
					Date:        email.Date,
					Attachments: attachments,
				}
				content = components.RenderReadView(emailData, a.width, a.viewport.View())
			}
		case composeView:
			content = lipgloss.Place(
				a.width,
				a.height-6,
				lipgloss.Center,
				lipgloss.Top,
				a.compose.View(),
			)
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
		content = components.RenderCentered(a.width, a.height, components.RenderConfirmDialog(deleteCount, a.deleteOption))
	}

	// Show search input overlay
	if a.searchMode {
		content = components.RenderCentered(a.width, a.height, components.RenderSearchInput(a.searchInput.View()))
	}

	// Show label picker overlay
	if a.showLabelPicker {
		content = a.labelPicker.View()
	}

	// Show command palette overlay
	if a.showCommandPalette {
		content = components.RenderCentered(a.width, a.height, a.commandPalette.View())
	}

	// Show summary dialog overlay
	if a.showSummary {
		content = components.RenderSummaryDialog(a.width, a.height, a.summaryText, a.summarySource)
	}

	// Show extract input dialog overlay
	if a.showExtractInput {
		content = components.RenderExtractInputDialog(a.width, a.height, a.extractInput.View())
	}

	// Show extract dialog overlay
	if a.showExtract && a.extractedEvent != nil {
		reminderStr := ""
		if a.extractedEvent.AlarmMinutesBefore > 0 {
			reminderStr = ReminderLabels[minutesToReminderIndex(a.extractedEvent.AlarmMinutesBefore)]
		}
		content = components.RenderExtractDialog(a.width, a.height, components.ExtractData{
			Title:     a.extractedEvent.Title,
			StartTime: a.extractedStart,
			EndTime:   a.extractedEnd,
			Location:  a.extractedEvent.Location,
			Reminder:  reminderStr,
			Provider:  a.extractedProvider,
		})
	}

	// Show extract edit dialog overlay (takes precedence over extract dialog)
	if a.showExtractEdit {
		content = components.RenderExtractEditDialog(a.width, a.height, components.ExtractEditData{
			TitleInput:    a.extractEditTitle.View(),
			DateInput:     a.extractEditDate.View(),
			StartInput:    a.extractEditStart.View(),
			EndInput:      a.extractEditEnd.View(),
			LocationInput: a.extractEditLocation.View(),
			ReminderIdx:   a.extractEditReminder,
			ReminderLabel: ReminderLabels[a.extractEditReminder],
			FocusIdx:      a.extractEditFocus,
			Provider:      a.extractedProvider,
		})
	}

	// Show attachment picker overlay
	if a.showAttachmentPicker {
		if email := a.mailList.SelectedEmail(); email != nil {
			var attachments []components.AttachmentInfo
			for _, att := range email.Attachments {
				attachments = append(attachments, components.AttachmentInfo{
					Filename:    att.Filename,
					ContentType: att.ContentType,
					Size:        att.Size,
				})
			}
			content = components.RenderAttachmentPicker(a.width, a.height, attachments, a.attachmentIdx)
		}
	}

	// Show file picker overlay (for compose attachments)
	if a.showFilePicker {
		content = a.filePicker.View()
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
		CurrentLabel:   a.currentLabel,
	}

	// Build status bar data
	statusData := components.StatusBarData{
		Width:          a.width,
		StatusMsg:      a.statusMsg,
		SearchMode:     a.searchMode,
		IsSearchResult: a.isSearchResult,
		IsListView:     a.view == listView,
		IsComposeView:  a.view == composeView,
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

func (a App) renderEmailContent(email mail.Email) string {
	body := email.Body
	if body == "" {
		body = email.Snippet
	}

	// Wrap text to fit viewport width (accounting for padding)
	wrapWidth := a.viewport.Width - 8
	if wrapWidth < 40 {
		wrapWidth = 40
	}

	contentStyle := lipgloss.NewStyle().
		Width(wrapWidth).
		PaddingLeft(4).
		PaddingRight(4)

	return contentStyle.Render(body)
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

func (a App) downloadAttachment(email *mail.Email, attachmentIdx int) tea.Cmd {
	return func() tea.Msg {
		if attachmentIdx < 0 || attachmentIdx >= len(email.Attachments) {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("invalid attachment index")}
		}

		att := email.Attachments[attachmentIdx]

		// Get downloads directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("cannot find home directory: %w", err)}
		}
		downloadsDir := filepath.Join(homeDir, "Downloads", "maily")

		// Ensure downloads directory exists
		if err := os.MkdirAll(downloadsDir, 0755); err != nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("cannot create downloads directory: %w", err)}
		}

		// Create IMAP client
		account := a.currentAccount()
		if account == nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("no account selected")}
		}

		imapClient, err := mail.NewIMAPClient(&account.Credentials)
		if err != nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("failed to connect: %w", err)}
		}
		defer imapClient.Close()

		// Fetch attachment content
		content, err := imapClient.FetchAttachment(a.currentLabel, email.UID, att.PartID, att.Encoding)
		if err != nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("failed to fetch attachment: %w", err)}
		}

		// Generate unique filename if file exists
		destPath := filepath.Join(downloadsDir, att.Filename)
		if _, err := os.Stat(destPath); err == nil {
			// File exists, add number suffix
			ext := filepath.Ext(att.Filename)
			base := att.Filename[:len(att.Filename)-len(ext)]
			for i := 1; ; i++ {
				destPath = filepath.Join(downloadsDir, fmt.Sprintf("%s (%d)%s", base, i, ext))
				if _, err := os.Stat(destPath); os.IsNotExist(err) {
					break
				}
			}
		}

		// Write file
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("failed to save file: %w", err)}
		}

		return attachmentDownloadedMsg{filename: att.Filename, path: destPath}
	}
}

func (a App) downloadAllAttachments(email *mail.Email) tea.Cmd {
	return func() tea.Msg {
		if len(email.Attachments) == 0 {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("no attachments")}
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("cannot find home directory: %w", err)}
		}
		downloadsDir := filepath.Join(homeDir, "Downloads", "maily")

		if err := os.MkdirAll(downloadsDir, 0755); err != nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("cannot create downloads directory: %w", err)}
		}

		account := a.currentAccount()
		if account == nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("no account selected")}
		}

		imapClient, err := mail.NewIMAPClient(&account.Credentials)
		if err != nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("failed to connect: %w", err)}
		}
		defer imapClient.Close()

		var downloaded []string
		for _, att := range email.Attachments {
			content, err := imapClient.FetchAttachment(a.currentLabel, email.UID, att.PartID, att.Encoding)
			if err != nil {
				return attachmentDownloadErrorMsg{err: fmt.Errorf("failed to fetch %s: %w", att.Filename, err)}
			}

			destPath := filepath.Join(downloadsDir, att.Filename)
			if _, err := os.Stat(destPath); err == nil {
				ext := filepath.Ext(att.Filename)
				base := att.Filename[:len(att.Filename)-len(ext)]
				for i := 1; ; i++ {
					destPath = filepath.Join(downloadsDir, fmt.Sprintf("%s (%d)%s", base, i, ext))
					if _, err := os.Stat(destPath); os.IsNotExist(err) {
						break
					}
				}
			}

			if err := os.WriteFile(destPath, content, 0644); err != nil {
				return attachmentDownloadErrorMsg{err: fmt.Errorf("failed to save %s: %w", att.Filename, err)}
			}
			downloaded = append(downloaded, att.Filename)
		}

		return attachmentDownloadedMsg{
			filename: fmt.Sprintf("%d files", len(downloaded)),
			path:     downloadsDir,
		}
	}
}
