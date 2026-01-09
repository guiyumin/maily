package ui

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maily/internal/mail"
	"maily/internal/ui/components"
)

// Package-level compiled regex for extracting email addresses
var emailExtractRegex = regexp.MustCompile(`<([^>]+)>`)

// maxQuotedBodyLen limits quoted body length to prevent performance issues
const maxQuotedBodyLen = 10000

// maxAttachmentSize is the Gmail attachment size limit (25MB)
const maxAttachmentSize = 25 * 1024 * 1024

// Focus fields
const (
	focusTo = iota
	focusSubject
	focusBody
	focusAttachments
	focusSend
	focusSaveDraft
	focusCancel
)

// numFocusFields is the total number of focus fields for cycling
const numFocusFields = 7

// Confirmation states
const (
	confirmNone = iota
	confirmSend
	confirmSaveDraft
	confirmCancel
)

// ComposeAttachment represents an attachment to be sent
type ComposeAttachment struct {
	Path        string
	Name        string
	Size        int64
	ContentType string
}

// ComposeModel handles email composition (reply/compose)
type ComposeModel struct {
	from            string
	toInput         textinput.Model
	subjectInput    textinput.Model
	body            textarea.Model
	width           int
	height          int
	focused         int
	isReply         bool
	replyEmail      *mail.Email // Original email being replied to
	confirming      int         // confirmNone, confirmSend, or confirmCancel
	confirmFocused  int         // 0 = Confirm button, 1 = Cancel button
	quotedBody      string      // stored quoted body for deferred initialization
	attachments     []ComposeAttachment
	totalAttachSize int64 // cumulative size of all attachments
	attachmentIdx   int   // currently selected attachment index
}

// OpenFilePickerMsg is sent when user wants to open the file picker
type OpenFilePickerMsg struct{}

// NewComposeModel creates a new compose model for a fresh email
func NewComposeModel(from string) ComposeModel {
	ti := textinput.New()
	ti.Placeholder = "recipient@example.com"
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 50

	si := textinput.New()
	si.Placeholder = "Subject"
	si.CharLimit = 200
	si.Width = 50

	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.CharLimit = 0
	ta.SetWidth(80)
	ta.SetHeight(10)

	return ComposeModel{
		from:         from,
		toInput:      ti,
		subjectInput: si,
		body:         ta,
		focused:      focusTo, // Start at To field for new compose
	}
}

// NewReplyModel creates a compose model for replying to an email
func NewReplyModel(from string, original *mail.Email) ComposeModel {
	// Determine who to reply to
	replyTo := original.From
	if original.ReplyTo != "" {
		replyTo = original.ReplyTo
	}

	ti := textinput.New()
	ti.SetValue(extractEmail(replyTo))
	ti.CharLimit = 200
	ti.Width = 50

	// Build subject
	subject := original.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}

	si := textinput.New()
	si.SetValue(subject)
	si.CharLimit = 200
	si.Width = 50

	ta := textarea.New()
	ta.Placeholder = "Type your reply..."
	ta.CharLimit = 0
	ta.SetWidth(80)
	ta.SetHeight(10)
	ta.Focus()

	// Build quoted body - will be set after first resize
	quotedBody := buildQuotedBody(original)

	return ComposeModel{
		from:         from,
		toInput:      ti,
		subjectInput: si,
		body:         ta,
		focused:      focusBody, // Start at body for reply
		isReply:      true,
		replyEmail:   original,
		quotedBody:   "\n\n" + quotedBody,
	}
}

// extractEmail extracts email address from "Name <email@example.com>" format
func extractEmail(s string) string {
	matches := emailExtractRegex.FindStringSubmatch(s)
	if len(matches) > 1 {
		return matches[1]
	}
	return s
}

// sanitizeControlChars removes ANSI escape sequences and control characters
// except for allowed whitespace (newline, tab). Properly handles UTF-8.
func sanitizeControlChars(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))

	i := 0
	for i < len(s) {
		// Check for ANSI escape sequence (ESC [ ... or ESC O ...)
		if i+1 < len(s) && s[i] == '\x1b' {
			// Skip the escape sequence
			j := i + 1
			if j < len(s) && (s[j] == '[' || s[j] == 'O') {
				j++
				// Skip until we find a letter (end of sequence)
				for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
					j++
				}
				if j < len(s) {
					j++ // Skip the final letter
				}
				i = j
				continue
			}
		}

		// Decode UTF-8 rune properly
		r, size := utf8.DecodeRuneInString(s[i:])
		// Allow printable characters, newlines, and tabs
		if unicode.IsPrint(r) || r == '\n' || r == '\t' {
			sb.WriteRune(r)
		}
		i += size
	}
	return sb.String()
}

// buildQuotedBody creates the quoted original email content
func buildQuotedBody(email *mail.Email) string {
	var sb strings.Builder

	// Sanitize From field to prevent escape injection
	sanitizedFrom := sanitizeControlChars(email.From)

	// Quote header
	dateStr := email.Date.Format("Mon, Jan 2, 2006 at 3:04 PM")
	sb.WriteString(fmt.Sprintf("On %s, %s wrote:\n", dateStr, sanitizedFrom))

	// Quote body with > prefix
	body := email.Body
	if body == "" {
		body = email.Snippet
	}

	// Sanitize body to prevent escape injection
	body = sanitizeControlChars(body)

	// Limit quoted body length to prevent performance issues
	if len(body) > maxQuotedBodyLen {
		body = body[:maxQuotedBodyLen] + "\n[... content truncated ...]"
	}

	lines := strings.Split(body, "\n")
	for _, line := range lines {
		sb.WriteString("> ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *ComposeModel) setSize(width, height int) {
	m.width = width
	m.height = height

	inputWidth := width - 20
	if inputWidth < 10 {
		inputWidth = 10
	}
	m.toInput.Width = inputWidth
	m.subjectInput.Width = inputWidth

	bodyWidth := width - 16
	if bodyWidth < 10 {
		bodyWidth = 10
	}
	m.body.SetWidth(bodyWidth)

	availableHeight := height - 6
	if availableHeight < 0 {
		availableHeight = height
	}
	bodyHeight := availableHeight - 11
	if bodyHeight < 5 {
		bodyHeight = 5
	}
	m.body.SetHeight(bodyHeight)

	m.applyDeferredReplyQuote()
}

func (m *ComposeModel) applyDeferredReplyQuote() {
	if !m.isReply || m.quotedBody == "" {
		return
	}
	m.body.SetValue(m.quotedBody)
	m.quotedBody = ""
	m.moveBodyCursorToTop()
}

func (m *ComposeModel) moveBodyCursorToTop() {
	for m.body.Line() > 0 || m.body.LineInfo().RowOffset > 0 {
		m.body.CursorUp()
	}
	m.body.CursorStart()
}

func isMouseEscapeKey(msg tea.KeyMsg) bool {
	if msg.Type != tea.KeyRunes || len(msg.Runes) < 3 {
		return false
	}
	if msg.Runes[0] != '\x1b' || msg.Runes[1] != '[' {
		return false
	}
	return msg.Runes[2] == '<' || msg.Runes[2] == 'M'
}

func (m ComposeModel) Init() tea.Cmd {
	if m.isReply {
		return textarea.Blink
	}
	return textinput.Blink
}

func (m *ComposeModel) focusField(field int) tea.Cmd {
	m.focused = field
	m.toInput.Blur()
	m.subjectInput.Blur()
	m.body.Blur()

	switch field {
	case focusTo:
		m.toInput.Focus()
		return textinput.Blink
	case focusSubject:
		m.subjectInput.Focus()
		return textinput.Blink
	case focusBody:
		m.body.Focus()
		return textarea.Blink
	case focusAttachments:
		// Reset attachment index if out of bounds
		if m.attachmentIdx >= len(m.attachments) {
			m.attachmentIdx = 0
		}
		return nil
	case focusSend, focusSaveDraft, focusCancel:
		// No input to focus, just visual
		return nil
	}
	return nil
}

// SendMsg is sent when user presses Enter on Send button
type SendMsg struct{}

// SaveDraftMsg is sent when user confirms save draft
type SaveDraftMsg struct{}

// CancelMsg is sent when user presses Enter on Cancel button
type CancelMsg struct{}

func (m ComposeModel) Update(msg tea.Msg) (ComposeModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if isMouseEscapeKey(msg) {
			return m, nil
		}
		// Handle confirmation dialogs
		if m.confirming != confirmNone {
			switch msg.String() {
			case "left", "right", "tab", "h", "l":
				// Toggle between Confirm and Cancel buttons
				m.confirmFocused = 1 - m.confirmFocused
				return m, nil
			case "y", "Y":
				// Quick confirm with Y
				action := m.confirming
				m.confirming = confirmNone
				m.confirmFocused = 0
				switch action {
				case confirmSend:
					return m, func() tea.Msg { return SendMsg{} }
				case confirmSaveDraft:
					return m, func() tea.Msg { return SaveDraftMsg{} }
				case confirmCancel:
					return m, func() tea.Msg { return CancelMsg{} }
				}
			case "enter":
				if m.confirmFocused == 0 {
					// Confirm button selected
					action := m.confirming
					m.confirming = confirmNone
					m.confirmFocused = 0
					switch action {
					case confirmSend:
						return m, func() tea.Msg { return SendMsg{} }
					case confirmSaveDraft:
						return m, func() tea.Msg { return SaveDraftMsg{} }
					case confirmCancel:
						return m, func() tea.Msg { return CancelMsg{} }
					}
				} else {
					// Cancel button selected
					m.confirming = confirmNone
					m.confirmFocused = 0
					return m, nil
				}
			case "n", "N", "esc":
				m.confirming = confirmNone
				m.confirmFocused = 0
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "enter":
			if m.focused == focusSend {
				m.confirming = confirmSend
				return m, nil
			}
			if m.focused == focusSaveDraft {
				m.confirming = confirmSaveDraft
				return m, nil
			}
			if m.focused == focusCancel {
				m.confirming = confirmCancel
				return m, nil
			}
		case "tab":
			// Cycle focus: To → Subject → Body → Attachments → Send → Save Draft → Cancel → To
			nextFocus := (m.focused + 1) % numFocusFields
			// Skip attachments if there are none
			if nextFocus == focusAttachments && len(m.attachments) == 0 {
				nextFocus = (nextFocus + 1) % numFocusFields
			}
			cmd = m.focusField(nextFocus)
			return m, cmd
		case "shift+tab":
			// Cycle focus backwards
			nextFocus := (m.focused + numFocusFields - 1) % numFocusFields
			// Skip attachments if there are none
			if nextFocus == focusAttachments && len(m.attachments) == 0 {
				nextFocus = (nextFocus + numFocusFields - 1) % numFocusFields
			}
			cmd = m.focusField(nextFocus)
			return m, cmd
		case "a", "A":
			// Open file picker to add attachment (only when not in text input)
			if m.focused != focusTo && m.focused != focusSubject && m.focused != focusBody {
				return m, func() tea.Msg { return OpenFilePickerMsg{} }
			}
		case "x", "d", "delete", "backspace":
			// Remove selected attachment when in attachments focus
			if m.focused == focusAttachments && len(m.attachments) > 0 {
				// Remove attachment at current index
				idx := m.attachmentIdx
				if idx >= 0 && idx < len(m.attachments) {
					m.totalAttachSize -= m.attachments[idx].Size
					m.attachments = append(m.attachments[:idx], m.attachments[idx+1:]...)
				}
				// If no more attachments, move focus to body
				if len(m.attachments) == 0 {
					cmd = m.focusField(focusBody)
					return m, cmd
				}
				// Adjust index if needed
				if m.attachmentIdx >= len(m.attachments) {
					m.attachmentIdx = len(m.attachments) - 1
				}
				return m, nil
			}
		case "left", "h":
			// Navigate attachments
			if m.focused == focusAttachments && m.attachmentIdx > 0 {
				m.attachmentIdx--
				return m, nil
			}
		case "right", "l":
			// Navigate attachments
			if m.focused == focusAttachments && m.attachmentIdx < len(m.attachments)-1 {
				m.attachmentIdx++
				return m, nil
			}
		}
	case tea.MouseMsg:
		// Ignore mouse events to prevent gibberish in textarea
		return m, nil
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil
	}

	if m.isReply && m.quotedBody != "" && m.body.Value() == "" {
		m.applyDeferredReplyQuote()
		cmds = append(cmds, textarea.Blink)
	}

	// Update the focused field
	switch m.focused {
	case focusTo:
		m.toInput, cmd = m.toInput.Update(msg)
		cmds = append(cmds, cmd)
	case focusSubject:
		m.subjectInput, cmd = m.subjectInput.Update(msg)
		cmds = append(cmds, cmd)
	case focusBody:
		m.body, cmd = m.body.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m ComposeModel) View() string {
	// Show confirmation dialog if confirming
	if m.confirming != confirmNone {
		return m.renderConfirmDialog()
	}

	// Styles for focused/unfocused fields
	focusedStyle := lipgloss.NewStyle().Foreground(components.Primary)
	labelStyle := lipgloss.NewStyle().Width(10)

	// From line (not editable)
	fromLine := labelStyle.Render("From:") + " " + m.from

	// To line
	toLabel := labelStyle.Render("To:")
	if m.focused == focusTo {
		toLabel = focusedStyle.Render(labelStyle.Render("To:"))
	}
	toLine := toLabel + " " + m.toInput.View()

	// Subject line
	subjectLabel := labelStyle.Render("Subject:")
	if m.focused == focusSubject {
		subjectLabel = focusedStyle.Render(labelStyle.Render("Subject:"))
	}
	subjectLine := subjectLabel + " " + m.subjectInput.View()

	// Clamp separator width to avoid panic on negative count
	separatorWidth := m.width - 16
	if separatorWidth < 0 {
		separatorWidth = 0
	}

	header := lipgloss.JoinVertical(
		lipgloss.Left,
		fromLine,
		toLine,
		subjectLine,
		strings.Repeat("─", separatorWidth),
	)

	// Body textarea
	bodySection := m.body.View()

	// Attachments section
	var attachSection string
	if len(m.attachments) > 0 {
		attachSection = m.renderAttachments()
	}

	// Button styles - both have border for consistent sizing
	btnStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder())
	btnFocusedStyle := btnStyle.
		Background(components.Primary).
		Foreground(lipgloss.Color("#FFFFFF")).
		BorderForeground(components.Primary).
		Bold(true)
	btnUnfocusedStyle := btnStyle.
		BorderForeground(components.TextDim)

	// Send button
	var sendBtn string
	if m.focused == focusSend {
		sendBtn = btnFocusedStyle.Render("Send")
	} else {
		sendBtn = btnUnfocusedStyle.Render("Send")
	}

	// Save Draft button
	var saveDraftBtn string
	if m.focused == focusSaveDraft {
		saveDraftBtn = btnFocusedStyle.Render("Save Draft")
	} else {
		saveDraftBtn = btnUnfocusedStyle.Render("Save Draft")
	}

	// Cancel button
	var cancelBtn string
	if m.focused == focusCancel {
		cancelBtn = btnFocusedStyle.Render("Cancel")
	} else {
		cancelBtn = btnUnfocusedStyle.Render("Cancel")
	}

	// Buttons row
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, sendBtn, "  ", saveDraftBtn, "  ", cancelBtn)

	// Compose everything
	var contentParts []string
	contentParts = append(contentParts, header, "", bodySection)
	if attachSection != "" {
		contentParts = append(contentParts, "", attachSection)
	}
	contentParts = append(contentParts, "", buttons)

	content := lipgloss.JoinVertical(lipgloss.Left, contentParts...)

	// Create a bordered container - clamp width to minimum of 1
	containerWidth := m.width - 8
	if containerWidth < 1 {
		containerWidth = 1
	}
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(components.Primary).
		Padding(1, 2).
		Width(containerWidth)

	// Title based on compose type
	titleText := " Compose "
	if m.isReply {
		titleText = " Reply "
	}
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(components.Primary).
		Render(titleText)

	box := containerStyle.Render(content)

	// Add title at top of border
	lines := strings.Split(box, "\n")
	if len(lines) > 0 {
		firstLine := lines[0]
		if len(firstLine) > 4 {
			lines[0] = firstLine[:2] + title + firstLine[2+lipgloss.Width(title):]
		}
		box = strings.Join(lines, "\n")
	}

	return box
}

// GetBody returns the composed email body
func (m ComposeModel) GetBody() string {
	return m.body.Value()
}

// sanitizeHeaderValue removes CR/LF and other control characters from header values
// to prevent header injection attacks
func sanitizeHeaderValue(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		// Skip CR, LF, and other control characters (except space/tab for headers)
		if r == '\r' || r == '\n' || (r < 32 && r != '\t') {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// GetTo returns the recipient email (sanitized to prevent header injection)
func (m ComposeModel) GetTo() string {
	return sanitizeHeaderValue(m.toInput.Value())
}

// GetSubject returns the subject (sanitized to prevent header injection)
func (m ComposeModel) GetSubject() string {
	return sanitizeHeaderValue(m.subjectInput.Value())
}

// GetOriginalEmail returns the original email being replied to
func (m ComposeModel) GetOriginalEmail() *mail.Email {
	return m.replyEmail
}

// AddAttachment adds a file attachment to the compose model
func (m *ComposeModel) AddAttachment(path, name, contentType string, size int64) error {
	if m.totalAttachSize+size > maxAttachmentSize {
		return fmt.Errorf("total attachments exceed 25MB limit (current: %s, adding: %s)",
			formatSize(m.totalAttachSize), formatSize(size))
	}

	m.attachments = append(m.attachments, ComposeAttachment{
		Path:        path,
		Name:        name,
		Size:        size,
		ContentType: contentType,
	})
	m.totalAttachSize += size
	return nil
}

// RemoveAttachment removes the attachment at the given index
func (m *ComposeModel) RemoveAttachment(idx int) {
	if idx >= 0 && idx < len(m.attachments) {
		m.totalAttachSize -= m.attachments[idx].Size
		m.attachments = append(m.attachments[:idx], m.attachments[idx+1:]...)
	}
}

// GetAttachments returns the list of attachments
func (m ComposeModel) GetAttachments() []ComposeAttachment {
	return m.attachments
}

// HasAttachments returns true if there are any attachments
func (m ComposeModel) HasAttachments() bool {
	return len(m.attachments) > 0
}

// formatSize formats a byte size for display
func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
	)
	switch {
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// renderAttachments renders the attachments list
func (m ComposeModel) renderAttachments() string {
	if len(m.attachments) == 0 {
		return ""
	}

	var parts []string

	// Header with total size
	headerStyle := lipgloss.NewStyle().Foreground(components.Muted)
	header := headerStyle.Render(fmt.Sprintf("Attachments (%d, %s):", len(m.attachments), formatSize(m.totalAttachSize)))
	parts = append(parts, header)

	// Attachment items
	for i, att := range m.attachments {
		icon := "📎"
		name := att.Name
		if utf8.RuneCountInString(name) > 25 {
			// Truncate by runes to avoid breaking UTF-8 characters
			runes := []rune(name)
			name = string(runes[:22]) + "..."
		}
		sizeStr := formatSize(att.Size)

		item := fmt.Sprintf("%s %s (%s)", icon, name, sizeStr)

		var style lipgloss.Style
		if m.focused == focusAttachments && i == m.attachmentIdx {
			// Highlighted attachment
			style = lipgloss.NewStyle().
				Bold(true).
				Foreground(components.Text).
				Background(components.Primary).
				Padding(0, 1)
			item = style.Render(item + " [x]")
		} else {
			style = lipgloss.NewStyle().
				Foreground(components.Text).
				Padding(0, 1)
			item = style.Render(item)
		}

		parts = append(parts, item)
	}

	// Add hint when focused on attachments
	if m.focused == focusAttachments {
		hintStyle := lipgloss.NewStyle().Foreground(components.Muted).Italic(true)
		parts = append(parts, hintStyle.Render("  ←/→ navigate • x remove • a add more"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderConfirmDialog renders a confirmation dialog
func (m ComposeModel) renderConfirmDialog() string {
	var title, message string

	switch m.confirming {
	case confirmSend:
		title = "Send Email?"
		message = "Are you sure you want to send this email?"
	case confirmSaveDraft:
		title = "Save Draft?"
		message = "Save this email as a draft?"
	case confirmCancel:
		title = "Discard Draft?"
		message = "Are you sure? Draft will not be saved."
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(components.Primary).
		MarginBottom(1)

	messageStyle := lipgloss.NewStyle().
		Foreground(components.Text).
		MarginBottom(1)

	// Button styles
	buttonStyle := lipgloss.NewStyle().
		Padding(0, 2).
		MarginRight(1)

	activeButtonStyle := buttonStyle.
		Background(components.Primary).
		Foreground(lipgloss.Color("#ffffff"))

	inactiveButtonStyle := buttonStyle.
		Background(components.Bg).
		Foreground(components.TextDim)

	// Render buttons based on focus
	var confirmBtn, cancelBtn string
	if m.confirmFocused == 0 {
		confirmBtn = activeButtonStyle.Render("Confirm")
		cancelBtn = inactiveButtonStyle.Render("Cancel")
	} else {
		confirmBtn = inactiveButtonStyle.Render("Confirm")
		cancelBtn = activeButtonStyle.Render("Cancel")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, confirmBtn, cancelBtn)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		titleStyle.Render(title),
		messageStyle.Render(message),
		"",
		buttons,
	)

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(components.Primary).
		Padding(1, 3).
		Width(50)

	dialog := dialogStyle.Render(content)

	// Center the dialog
	return lipgloss.Place(
		m.width,
		m.height-6,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	)
}
