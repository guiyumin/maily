package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cocomail/internal/gmail"
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
	emails   []gmail.Email
	cursor   int
	width    int
	height   int
	keyMap   MailListKeyMap
	selected *gmail.Email
}

func NewMailList() MailList {
	return MailList{
		emails: []gmail.Email{},
		cursor: 0,
		keyMap: DefaultMailListKeyMap,
	}
}

func (m *MailList) SetEmails(emails []gmail.Email) {
	m.emails = emails
	if m.cursor >= len(emails) {
		m.cursor = max(0, len(emails)-1)
	}
}

func (m *MailList) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m MailList) SelectedEmail() *gmail.Email {
	if len(m.emails) == 0 || m.cursor < 0 || m.cursor >= len(m.emails) {
		return nil
	}
	return &m.emails[m.cursor]
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

	visibleHeight := m.height - 4
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

func (m MailList) renderEmailLine(email gmail.Email, selected bool) string {
	maxWidth := m.width - 10
	if maxWidth < 40 {
		maxWidth = 80
	}

	from := truncate(extractName(email.From), 20)
	subject := truncate(email.Subject, maxWidth-35)
	date := formatDate(email.Date)

	unreadMarker := " "
	if email.Unread {
		unreadMarker = "●"
	}

	line := fmt.Sprintf("%s %-20s │ %-*s │ %s",
		unreadMarker,
		from,
		maxWidth-35,
		subject,
		date,
	)

	if selected {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F9FAFB")).
			Background(lipgloss.Color("#7C3AED")).
			Width(m.width - 4).
			Render(line)
	}

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB")).
		Width(m.width - 4)

	if email.Unread {
		style = style.Bold(true)
	}

	return style.Render(line)
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
		return t.Format("15:04")
	}
	if t.Year() == now.Year() {
		return t.Format("Jan 02")
	}
	return t.Format("02/01/06")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
