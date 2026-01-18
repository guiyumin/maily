package components

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/emersion/go-imap/v2"

	"maily/internal/mail"
)

type MailListKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Delete   key.Binding
	MarkRead key.Binding
	Refresh  key.Binding
}

var DefaultMailListKeyMap = MailListKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	MarkRead: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "mark read/unread"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
}

type MailList struct {
	emails        []mail.Email
	cursor        int
	width         int
	height        int
	keyMap        MailListKeyMap
	selectionMode bool
	selections    map[imap.UID]bool
}

func NewMailList() MailList {
	return MailList{
		emails: []mail.Email{},
		cursor: 0,
		keyMap: DefaultMailListKeyMap,
	}
}

func (m *MailList) SetEmails(emails []mail.Email) {
	m.emails = emails
	if m.cursor >= len(emails) {
		m.cursor = max(0, len(emails)-1)
	}
}

func (m MailList) Emails() []mail.Email {
	return m.emails
}

func (m *MailList) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *MailList) RemoveCurrent() {
	if len(m.emails) == 0 || m.cursor < 0 || m.cursor >= len(m.emails) {
		return
	}
	m.emails = append(m.emails[:m.cursor], m.emails[m.cursor+1:]...)
	if m.cursor >= len(m.emails) && m.cursor > 0 {
		m.cursor--
	}
}

func (m *MailList) RemoveByUID(uid imap.UID) {
	for i, email := range m.emails {
		if email.UID == uid {
			m.emails = append(m.emails[:i], m.emails[i+1:]...)
			if m.cursor >= len(m.emails) && m.cursor > 0 {
				m.cursor--
			}
			return
		}
	}
}

func (m *MailList) MarkAsRead(uid imap.UID) {
	for i := range m.emails {
		if m.emails[i].UID == uid {
			m.emails[i].Unread = false
			return
		}
	}
}

func (m *MailList) MarkAsUnread(uid imap.UID) {
	for i := range m.emails {
		if m.emails[i].UID == uid {
			m.emails[i].Unread = true
			return
		}
	}
}

// UpdateEmailBody updates the body content for an email that was loaded without body
func (m *MailList) UpdateEmailBody(uid imap.UID, bodyHTML, snippet string) {
	for i := range m.emails {
		if m.emails[i].UID == uid {
			m.emails[i].BodyHTML = bodyHTML
			m.emails[i].Snippet = snippet
			return
		}
	}
}

func (m *MailList) ScrollUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *MailList) ScrollDown() {
	if m.cursor < len(m.emails)-1 {
		m.cursor++
	}
}

func (m MailList) SelectedEmail() *mail.Email {
	if len(m.emails) == 0 || m.cursor < 0 || m.cursor >= len(m.emails) {
		return nil
	}
	return &m.emails[m.cursor]
}

func (m MailList) Cursor() int {
	return m.cursor
}

func (m *MailList) SetSelectionMode(enabled bool) {
	m.selectionMode = enabled
	if !enabled {
		m.selections = nil
	}
}

func (m *MailList) SetSelections(selections map[imap.UID]bool) {
	m.selections = selections
}

func (m MailList) Update(msg tea.Msg) (MailList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keyMap.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keyMap.Down):
			if m.cursor < len(m.emails)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m MailList) View() string {
	if len(m.emails) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Padding(2).
			Render("No emails to display")
	}

	var b strings.Builder

	visibleHeight := m.height - 1 // use height directly, SetSize already accounts for chrome
	if visibleHeight < 1 {
		visibleHeight = 10
	}

	start := 0
	if m.cursor >= visibleHeight {
		start = m.cursor - visibleHeight + 1
	}

	end := start + visibleHeight
	if end > len(m.emails) {
		end = len(m.emails)
	}

	for i := start; i < end; i++ {
		email := m.emails[i]
		line := m.renderEmailLine(email, i == m.cursor)
		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m MailList) renderEmailLine(email mail.Email, isCursor bool) string {
	dateWidth := 12
	fromWidth := 20
	statusWidth := 5
	attachWidth := 3 // for 📎 icon + space
	checkboxWidth := 0
	rightPadding := 4
	spacing := 4 // spaces between columns

	// Add checkbox width if in selection mode
	if m.selectionMode {
		checkboxWidth = 5
	}

	availableWidth := m.width - statusWidth - attachWidth - checkboxWidth - fromWidth - dateWidth - spacing - rightPadding
	if availableWidth < 20 {
		availableWidth = 20
	}

	from := truncate(extractName(email.From), fromWidth)
	subject := truncate(email.Subject, availableWidth)
	date := formatDate(email.Date)

	// Checkbox for selection mode
	isSelected := m.selections[email.UID]
	var checkbox string
	if m.selectionMode {
		if isSelected {
			checkbox = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render(" [✓] ")
		} else {
			checkbox = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(" [ ] ")
		}
	}

	// Status indicator - show read/unread
	var status string
	if email.Unread {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Render("  ●  ")
	} else {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("  ○  ")
	}

	// Attachment indicator
	var attachIcon string
	if len(email.Attachments) > 0 {
		attachIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Render("📎 ")
	} else {
		attachIcon = "   "
	}

	fromStyle := lipgloss.NewStyle().Width(fromWidth)
	subjectStyle := lipgloss.NewStyle().Width(availableWidth)
	dateStyle := lipgloss.NewStyle().Width(dateWidth).Align(lipgloss.Right)

	line := fromStyle.Render(from) + "  " + subjectStyle.Render(subject) + "  " + dateStyle.Render(date)

	lineStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB"))

	if isCursor {
		lineStyle = lineStyle.
			Bold(true).
			Background(lipgloss.Color("#7C3AED"))
	} else if m.selectionMode && isSelected {
		lineStyle = lineStyle.
			Foreground(lipgloss.Color("#10B981"))
	} else if email.Unread {
		lineStyle = lineStyle.Bold(true)
	}

	return checkbox + status + attachIcon + lineStyle.Render(line)
}

func extractName(from string) string {
	if idx := strings.Index(from, "<"); idx > 0 {
		return strings.TrimSpace(from[:idx])
	}
	return from
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func formatDate(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return "Today " + t.Format("15:04")
	}
	return t.Format("Jan 02, 2006")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
