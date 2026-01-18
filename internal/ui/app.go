package ui

import (
	"fmt"
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
	"maily/internal/client"
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
	serverClient    *client.Client                // connection to maily server
	emailCache      map[string][]mail.Email       // key: "accountIdx:label"
	diskCache       *cache.Cache                  // persistent disk cache (fallback only)
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
	aiClient        *ai.Client
	showSummary     bool
	summaryText     string
	summarySource   string // which AI provider was used
	summaryViewport viewport.Model
	showAISetup     bool // show AI setup confirmation dialog
	LaunchConfigUI  bool // signal to launch config TUI after exit

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
	extractEditNotes     textinput.Model
	extractEditReminder  int // index into reminderOptions: 0=none, 1=5min, 2=10min, 3=15min, 4=30min, 5=1hr
	extractEditFocus     int // 0=title, 1=date, 2=start, 3=end, 4=location, 5=notes, 6=reminder, 7=save, 8=cancel

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

type markUnreadCompleteMsg struct {
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

type emailBodyLoadedMsg struct {
	uid          imap.UID
	bodyHTML     string
	snippet      string
	accountEmail string
	mailbox      string
}

type emailBodyErrorMsg struct {
	uid          imap.UID
	err          error
	accountEmail string
	mailbox      string
}

type serverRefreshCompleteMsg struct {
	emails       []mail.Email
	accountEmail string
	mailbox      string
	err          error
}

const (
	autoRefreshInterval  = 5 * time.Minute
	cacheFreshnessWindow = 10 * time.Minute
)

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

	// Connect to server (ignore error, will fall back to direct access)
	serverClient, _ := client.Connect()

	// Initialize disk cache as fallback (ignore error)
	diskCache, _ := cache.New()

	// Initialize calendar client (ignore error, will just skip calendar features)
	calClient, _ := calendar.NewClient()

	return App{
		store:          store,
		cfg:            cfg,
		accountIdx:     0,
		serverClient:   serverClient,
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
					// Pass current email for context (helps resolve "them", "the meeting", etc.)
					email := a.mailList.SelectedEmail()
					return a, tea.Batch(a.spinner.Tick, a.parseManualEvent(input, email))
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
				a.extractEditFocus = (a.extractEditFocus + 1) % 9
				a.updateExtractEditFocus()
				return a, nil
			case "shift+tab":
				a.extractEditFocus = (a.extractEditFocus - 1 + 9) % 9
				a.updateExtractEditFocus()
				return a, nil
			case "up", "k":
				// For reminder field, cycle through options
				if a.extractEditFocus == 6 {
					a.extractEditReminder = (a.extractEditReminder - 1 + 6) % 6
					return a, nil
				}
			case "down", "j":
				// For reminder field, cycle through options
				if a.extractEditFocus == 6 {
					a.extractEditReminder = (a.extractEditReminder + 1) % 6
					return a, nil
				}
			case "enter":
				switch a.extractEditFocus {
				case 7: // Save button
					if err := a.applyExtractEdits(); err != nil {
						a.statusMsg = fmt.Sprintf("Invalid input: %v", err)
						return a, nil
					}
					a.showExtractEdit = false
					a.statusMsg = "Changes saved"
					return a, nil
				case 8: // Cancel button
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
				case 5:
					a.extractEditNotes, cmd = a.extractEditNotes.Update(msg)
				// case 6: reminder uses up/down, no text input
				// case 7, 8: buttons, no text input
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

		// Handle summary dialog scrolling
		if a.showSummary {
			switch msg.String() {
			case "j", "down":
				a.summaryViewport.ScrollDown(1)
				return a, nil
			case "k", "up":
				a.summaryViewport.ScrollUp(1)
				return a, nil
			case "g":
				a.summaryViewport.SetYOffset(0)
				return a, nil
			case "G":
				a.summaryViewport.SetYOffset(a.summaryViewport.TotalLineCount())
				return a, nil
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			// Close server client
			if a.serverClient != nil {
				a.serverClient.Close()
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
			} else if a.showAISetup {
				a.showAISetup = false
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
			// Handle AI setup confirmation dialog
			if a.showAISetup {
				a.LaunchConfigUI = true
				return a, tea.Quit
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
					// Create fresh viewport for each email to avoid state issues
					emailHeaderHeight := 6
					if len(email.Attachments) > 0 {
						emailHeaderHeight = 7
					}
					vpHeight := max(5, a.height-10-emailHeaderHeight)
					a.viewport = viewport.New(a.width-8, vpHeight)
					a.viewport.Style = lipgloss.NewStyle().Padding(1, 4, 3, 4)

					// Check if body needs to be fetched
					if email.BodyHTML == "" && email.Snippet == "" {
						a.viewport.SetContent("Loading email content...")
						// Trigger async body fetch
						cmd := a.fetchEmailBody(email.UID)
						if email.Unread {
							uid := email.UID
							account := a.currentAccount()
							label := a.currentLabel
							serverClient := a.serverClient
							a.mailList.MarkAsRead(uid)
							go func() {
								if serverClient != nil && account != nil {
									_ = serverClient.MarkRead(account.Credentials.Email, label, uid)
								}
							}()
						}
						return a, cmd
					}

					a.viewport.SetContent(a.renderEmailContent(*email))

					if email.Unread {
						uid := email.UID
						account := a.currentAccount()
						label := a.currentLabel
						serverClient := a.serverClient
						// Update in-memory state immediately for responsive UI
						a.mailList.MarkAsRead(uid)
						go func() {
							if serverClient != nil && account != nil {
								_ = serverClient.MarkRead(account.Credentials.Email, label, uid)
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
			// Shift+R for refresh - direct IMAP metadata-only refresh
			if a.state == stateReady && !a.isSearchResult && a.view == listView {
				a.state = stateLoading
				a.statusMsg = "Refreshing..."
				return a, tea.Batch(a.spinner.Tick, a.refreshFromIMAP())
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
						a.showAISetup = true
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
					a.showAISetup = true
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
		case "u":
			// Mark as unread (read view only)
			if a.state == stateReady && a.view == readView && !a.confirmDelete {
				email := a.mailList.SelectedEmail()
				if email != nil {
					uid := email.UID
					a.state = stateLoading
					a.statusMsg = "Marking as unread..."
					return a, tea.Batch(a.spinner.Tick, a.markSingleAsUnread(uid))
				}
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
			// Block account switching when any dialog is open
			if len(a.store.Accounts) > 1 && !a.confirmDelete && !a.isSearchResult && !a.showLabelPicker &&
				!a.showExtractEdit && !a.showExtract && !a.showExtractInput && !a.showSummary && !a.showAISetup {
				// Save current emails to cache (only if not in error state)
				if a.state != stateError {
					if emails := a.mailList.Emails(); len(emails) > 0 {
						cacheKey := fmt.Sprintf("%d:%s", a.accountIdx, a.currentLabel)
						a.emailCache[cacheKey] = emails
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
					a.mailList.SetEmails(cached)
					a.state = stateReady
					labelName := components.GetLabelDisplayName(a.currentLabel)
					a.statusMsg = fmt.Sprintf("%s: %d emails", labelName, len(cached))
					return a, nil
				}

				// Load from server/disk cache
				a.state = stateLoading
				a.emailLimit = uint32(a.cfg.MaxEmails)
				a.mailList.SetEmails(nil)
				a.statusMsg = "Loading..."
				return a, tea.Batch(a.spinner.Tick, a.loadCachedEmails())
			}
		}

	case tea.MouseMsg:
		// Handle summary dialog mouse scroll
		if a.showSummary {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				a.summaryViewport.ScrollUp(3)
				return a, nil
			case tea.MouseButtonWheelDown:
				a.summaryViewport.ScrollDown(3)
				return a, nil
			}
		}

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
		// Viewport height depends on view (readView has email header)
		if a.view == readView {
			a.viewport.Height = msg.Height - 16 // account for app header, email header, status bar
		} else {
			a.viewport.Height = msg.Height - 8
		}
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
			if a.diskCache.IsFresh(currentEmail, a.currentLabel, cacheFreshnessWindow) {
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
			if a.diskCache != nil && a.diskCache.IsFresh(currentEmail, a.currentLabel, cacheFreshnessWindow) {
				a.state = stateReady
				a.statusMsg = fmt.Sprintf("%s: %d emails", labelName, len(msg.emails))
			} else {
				// Cache is stale, show emails but keep loading state for background fetch
				a.state = stateReady
				a.statusMsg = fmt.Sprintf("%s: %d emails", labelName, len(msg.emails))
			}
		}
		return a, a.loadLabels()

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
		if a.diskCache != nil && currentEmail != "" {
			uidValidity := msg.uidValidity
			label := a.currentLabel
			go func(email, mailbox string, uidValidity uint32) {
				if uidValidity == 0 {
					if meta, err := a.diskCache.LoadMetadata(email, mailbox); err == nil && meta != nil {
						uidValidity = meta.UIDValidity
					}
				}
				meta := &cache.Metadata{UIDValidity: uidValidity, LastSync: time.Now()}
				_ = a.diskCache.SaveMetadata(email, mailbox, meta)
			}(currentEmail, label, uidValidity)
		}

	case serverRefreshCompleteMsg:
		// Ignore messages from other accounts/mailboxes (stale messages after switching)
		currentAccount := a.currentAccount()
		currentEmail := ""
		if currentAccount != nil {
			currentEmail = currentAccount.Credentials.Email
		}
		if msg.accountEmail != currentEmail || msg.mailbox != a.currentLabel {
			return a, nil
		}
		if msg.err != nil {
			a.state = stateError
			a.err = msg.err
			a.statusMsg = fmt.Sprintf("Refresh failed: %v", msg.err)
			return a, nil
		}
		a.mailList.SetEmails(msg.emails)
		cacheKey := fmt.Sprintf("%d:%s", a.accountIdx, a.currentLabel)
		a.emailCache[cacheKey] = msg.emails
		a.state = stateReady
		labelName := components.GetLabelDisplayName(a.currentLabel)
		a.statusMsg = fmt.Sprintf("%s: %d emails (refreshed)", labelName, len(msg.emails))

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

	case markUnreadCompleteMsg:
		a.state = stateReady
		a.view = listView
		a.mailList.MarkAsUnread(msg.uid)
		a.statusMsg = "Marked as unread"

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
		// Initialize summary viewport for scrolling
		dialogHeight := min(a.height-10, 20) // Max height for summary content
		vpWidth := min(a.width-30, 100)
		a.summaryViewport = viewport.New(vpWidth, dialogHeight)
		a.summaryViewport.MouseWheelEnabled = true
		// Wrap text with hanging indent for list items
		wrappedContent := components.WrapWithHangingIndent(msg.summary, vpWidth)
		a.summaryViewport.SetContent(wrappedContent)

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

	case emailBodyLoadedMsg:
		// Skip UI update if account/mailbox changed since fetch started
		currentAccount := a.currentAccount()
		if currentAccount == nil || msg.accountEmail != currentAccount.Credentials.Email || msg.mailbox != a.currentLabel {
			return a, nil
		}
		// Update the email body in the mail list
		a.mailList.UpdateEmailBody(msg.uid, msg.bodyHTML, msg.snippet)
		// Re-render if we're still viewing this email
		if a.view == readView {
			if email := a.mailList.SelectedEmail(); email != nil && email.UID == msg.uid {
				a.viewport.SetContent(a.renderEmailContent(*email))
			}
		}

	case emailBodyErrorMsg:
		// Skip error display if account/mailbox changed since fetch started
		currentAccount := a.currentAccount()
		if currentAccount == nil || msg.accountEmail != currentAccount.Credentials.Email || msg.mailbox != a.currentLabel {
			return a, nil
		}
		// Only show error if still viewing the same email
		if a.view == readView {
			if email := a.mailList.SelectedEmail(); email != nil && email.UID == msg.uid {
				a.viewport.SetContent(fmt.Sprintf("Failed to load email content: %v", msg.err))
			}
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

	// Show AI setup dialog overlay
	if a.showAISetup {
		content = components.RenderCentered(a.width, a.height, components.RenderAISetupDialog())
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
		content = components.RenderSummaryDialog(a.width, a.height, a.summaryViewport.View(), a.summarySource, a.summaryViewport.TotalLineCount() > a.summaryViewport.Height)
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
			NotesInput:    a.extractEditNotes.View(),
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

	header := components.RenderHeader(headerData)
	status := components.RenderStatusBar(statusData)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		content,
		status,
	)
}

func (a App) renderEmailContent(email mail.Email) string {
	body := email.BodyHTML
	if body == "" {
		body = email.Snippet
	}

	// Wrap text to fit viewport width (accounting for padding)
	wrapWidth := a.viewport.Width - 8
	if wrapWidth < 40 {
		wrapWidth = 40
	}

	// Render HTML body with glamour
	rendered := components.RenderHTMLBody(body, wrapWidth)

	contentStyle := lipgloss.NewStyle().
		PaddingLeft(4).
		PaddingRight(4)

	return contentStyle.Render(rendered)
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
	account := a.currentAccount()
	serverClient := a.serverClient
	mailbox := a.currentLabel

	return func() tea.Msg {
		if attachmentIdx < 0 || attachmentIdx >= len(email.Attachments) {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("invalid attachment index")}
		}

		if serverClient == nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("server unavailable")}
		}

		if account == nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("no account selected")}
		}

		att := email.Attachments[attachmentIdx]

		// Server downloads and saves the file, returns the path
		filePath, err := serverClient.DownloadAttachment(
			account.Credentials.Email,
			mailbox,
			email.UID,
			att.PartID,
			att.Filename,
			att.Encoding,
		)
		if err != nil {
			return attachmentDownloadErrorMsg{err: err}
		}

		return attachmentDownloadedMsg{filename: att.Filename, path: filePath}
	}
}

func (a App) downloadAllAttachments(email *mail.Email) tea.Cmd {
	account := a.currentAccount()
	serverClient := a.serverClient
	mailbox := a.currentLabel

	return func() tea.Msg {
		if len(email.Attachments) == 0 {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("no attachments")}
		}

		if serverClient == nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("server unavailable")}
		}

		if account == nil {
			return attachmentDownloadErrorMsg{err: fmt.Errorf("no account selected")}
		}

		var downloaded []string
		var lastPath string
		for _, att := range email.Attachments {
			filePath, err := serverClient.DownloadAttachment(
				account.Credentials.Email,
				mailbox,
				email.UID,
				att.PartID,
				att.Filename,
				att.Encoding,
			)
			if err != nil {
				return attachmentDownloadErrorMsg{err: fmt.Errorf("failed to download %s: %w", att.Filename, err)}
			}
			downloaded = append(downloaded, att.Filename)
			lastPath = filePath
		}

		// Get the downloads directory from the last file path
		downloadsDir := lastPath
		if len(downloaded) > 0 {
			// Extract directory from path
			for i := len(lastPath) - 1; i >= 0; i-- {
				if lastPath[i] == '/' {
					downloadsDir = lastPath[:i]
					break
				}
			}
		}

		return attachmentDownloadedMsg{
			filename: fmt.Sprintf("%d files", len(downloaded)),
			path:     downloadsDir,
		}
	}
}
