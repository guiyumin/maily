package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maily/internal/i18n"
)

// providerIDs defines available email providers
var providerIDs = []string{"gmail", "yahoo", "qq"}

// getProviderName returns the translated name for a provider
func getProviderName(id string) string {
	return i18n.T("provider." + id)
}

// getProviderDesc returns the translated description for a provider
func getProviderDesc(id string) string {
	return i18n.T("provider." + id + "_desc")
}

type ProviderSelector struct {
	cursor   int
	selected string
	width    int
	height   int
}

func NewProviderSelector() ProviderSelector {
	return ProviderSelector{
		cursor: 0,
	}
}

func (m ProviderSelector) Init() tea.Cmd {
	return nil
}

func (m ProviderSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(providerIDs)-1 {
				m.cursor++
			}

		case "enter":
			m.selected = providerIDs[m.cursor]
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m ProviderSelector) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		MarginBottom(1)

	itemStyle := lipgloss.NewStyle()

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		MarginTop(1)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1, 3)

	title := titleStyle.Render(i18n.T("provider.select_title"))

	var items strings.Builder
	for i, id := range providerIDs {
		cursor := "  "
		style := itemStyle
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}
		line := fmt.Sprintf("%s%s", cursor, style.Render(getProviderName(id)))
		desc := descStyle.Render("  " + getProviderDesc(id))
		items.WriteString(line + desc + "\n")
	}

	hint := hintStyle.Render(i18n.T("provider.hint"))

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		items.String(),
		hint,
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		boxStyle.Render(content),
	)
}

func (m ProviderSelector) Selected() string {
	return m.selected
}
