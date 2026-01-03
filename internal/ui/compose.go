package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maily/internal/mail"
	"maily/internal/ui/components"
)

// Focus fields
const (
	focusTo = iota
	focusSubject
	focusBody
	focusSend
	focusSaveDraft
	focusCancel
)

// Confirmation states
const (
	confirmNone = iota
	confirmSend
	confirmSaveDraft
	confirmCancel
)

// ComposeModel handles email composition (reply/compose)
type ComposeModel struct {
	from         string
	toInput      textinput.Model
	subjectInput textinput.Model
	body         textarea.Model
	width        int
	height       int
	focused      int
	isReply      bool
	replyEmail   *mail.Email // Original email being replied to
	confirming   int         // confirmNone, confirmSend, or confirmCancel
	quotedBody   string      // stored quoted body for deferred initialization
}

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
	re := regexp.MustCompile(`<([^>]+)>`)
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		return matches[1]
	}
	return s
}

// buildQuotedBody creates the quoted original email content
func buildQuotedBody(email *mail.Email) string {
	var sb strings.Builder

	// Quote header
	dateStr := email.Date.Format("Mon, Jan 2, 2006 at 3:04 PM")
	sb.WriteString(fmt.Sprintf("On %s, %s wrote:\n", dateStr, email.From))

	// Quote body with > prefix
	body := email.Body
	if body == "" {
		body = email.Snippet
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
			case "y", "Y", "enter":
				switch m.confirming {
				case confirmSend:
					m.confirming = confirmNone
					return m, func() tea.Msg { return SendMsg{} }
				case confirmSaveDraft:
					m.confirming = confirmNone
					return m, func() tea.Msg { return SaveDraftMsg{} }
				case confirmCancel:
					m.confirming = confirmNone
					return m, func() tea.Msg { return CancelMsg{} }
				}
			case "n", "N", "esc":
				m.confirming = confirmNone
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
			// Cycle focus: To → Subject → Body → Send → Save Draft → Cancel → To
			nextFocus := (m.focused + 1) % 6
			cmd = m.focusField(nextFocus)
			return m, cmd
		case "shift+tab":
			// Cycle focus backwards
			nextFocus := (m.focused + 5) % 6
			cmd = m.focusField(nextFocus)
			return m, cmd
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

	header := lipgloss.JoinVertical(
		lipgloss.Left,
		fromLine,
		toLine,
		subjectLine,
		strings.Repeat("─", m.width-16),
	)

	// Body textarea
	bodySection := m.body.View()

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
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		bodySection,
		"",
		buttons,
	)

	// Create a bordered container
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(components.Primary).
		Padding(1, 2).
		Width(m.width - 8)

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

// GetTo returns the recipient email
func (m ComposeModel) GetTo() string {
	return m.toInput.Value()
}

// GetSubject returns the subject
func (m ComposeModel) GetSubject() string {
	return m.subjectInput.Value()
}

// GetOriginalEmail returns the original email being replied to
func (m ComposeModel) GetOriginalEmail() *mail.Email {
	return m.replyEmail
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

	hintStyle := lipgloss.NewStyle().
		Foreground(components.TextDim)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		titleStyle.Render(title),
		messageStyle.Render(message),
		"",
		hintStyle.Render("Press Y to confirm, N to cancel"),
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
