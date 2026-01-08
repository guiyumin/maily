package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"maily/config"
)

var (
	cfgTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	cfgLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA")).Width(20)
	cfgValueStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F9FAFB"))
	cfgMutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	cfgSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#7C3AED"))
	cfgHintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	cfgSuccessStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	cfgDangerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
)

type configRow struct {
	key      string
	label    string
	value    string
	aiIndex  int
	editable bool
	isSecret bool
	isAction bool
}

type ConfigTUI struct {
	cfg      config.Config
	rows     []configRow
	cursor   int
	editMode bool
	editing  bool // actively editing a field
	input    textinput.Model
	dirty    bool
	confirm  bool
}

func NewConfigTUI() ConfigTUI {
	cfg, _ := config.Load()
	m := ConfigTUI{cfg: cfg}
	m.buildRows()
	return m
}

func (m *ConfigTUI) buildRows() {
	m.rows = []configRow{
		{key: "max_emails", label: "max_emails", value: fmt.Sprintf("%d", m.cfg.MaxEmails), aiIndex: -1, editable: true},
		{key: "default_label", label: "default_label", value: m.cfg.DefaultLabel, aiIndex: -1, editable: true},
		{key: "theme", label: "theme", value: m.cfg.Theme, aiIndex: -1, editable: true},
	}

	for i, acc := range m.cfg.AIAccounts {
		prefix := fmt.Sprintf("ai[%d]", i)

		maskedKey := acc.APIKey
		if len(maskedKey) > 8 {
			maskedKey = maskedKey[:4] + "..." + maskedKey[len(maskedKey)-4:]
		}

		m.rows = append(m.rows,
			configRow{key: "name", label: prefix + ".name", value: acc.Name, aiIndex: i, editable: true},
			configRow{key: "base_url", label: prefix + ".base_url", value: acc.BaseURL, aiIndex: i, editable: true},
			configRow{key: "model", label: prefix + ".model", value: acc.Model, aiIndex: i, editable: true},
			configRow{key: "api_key", label: prefix + ".api_key", value: maskedKey, aiIndex: i, editable: true, isSecret: true},
			configRow{key: "delete", label: prefix + ".delete", value: "(enter to delete)", aiIndex: i, isAction: true},
		)
	}

	// Only add action rows in edit mode
	if m.editMode {
		m.rows = append(m.rows,
			configRow{key: "add", label: "+ add ai account", isAction: true},
			configRow{key: "save", label: "[Save]", isAction: true},
			configRow{key: "cancel", label: "[Cancel]", isAction: true},
		)
	}
}

func (m ConfigTUI) Init() tea.Cmd {
	return nil
}

func (m ConfigTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirm {
			return m.updateConfirm(msg)
		}
		if m.editing {
			return m.updateEditing(msg)
		}
		if m.editMode {
			return m.updateEditMode(msg)
		}
		return m.updateReadOnly(msg)
	}
	return m, nil
}

func (m ConfigTUI) updateReadOnly(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "e":
		m.editMode = true
		m.buildRows()
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m ConfigTUI) updateEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "enter":
		return m.handleSelect()
	case "q", "esc", "ctrl+c":
		if m.dirty {
			m.confirm = true
			return m, nil
		}
		m.editMode = false
		m.buildRows()
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
	}
	return m, nil
}

func (m ConfigTUI) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.saveField()
	case "esc":
		m.editing = false
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m ConfigTUI) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "s":
		m.cfg.Save()
		m.dirty = false
		m.editMode = false
		m.confirm = false
		m.buildRows()
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
	case "n", "d":
		m.dirty = false
		m.editMode = false
		m.confirm = false
		// Reload original config
		m.cfg, _ = config.Load()
		m.buildRows()
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
	case "esc", "c":
		m.confirm = false
	}
	return m, nil
}

func (m ConfigTUI) handleSelect() (tea.Model, tea.Cmd) {
	row := m.rows[m.cursor]

	switch row.key {
	case "add":
		m.cfg.AIAccounts = append(m.cfg.AIAccounts, config.AIAccount{})
		m.dirty = true
		m.buildRows()
		return m, nil

	case "save":
		m.cfg.Save()
		m.dirty = false
		m.editMode = false
		m.buildRows()
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
		return m, nil

	case "cancel":
		if m.dirty {
			m.confirm = true
			return m, nil
		}
		m.editMode = false
		m.buildRows()
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
		return m, nil

	case "delete":
		if row.aiIndex >= 0 && row.aiIndex < len(m.cfg.AIAccounts) {
			m.cfg.AIAccounts = append(m.cfg.AIAccounts[:row.aiIndex], m.cfg.AIAccounts[row.aiIndex+1:]...)
			m.dirty = true
			m.buildRows()
			if m.cursor >= len(m.rows) {
				m.cursor = len(m.rows) - 1
			}
		}
		return m, nil
	}

	if row.editable {
		m.editing = true
		m.input = textinput.New()
		m.input.Focus()
		m.input.CharLimit = 200
		m.input.Width = 40

		if row.isSecret {
			m.input.Placeholder = "enter value"
			m.input.EchoMode = textinput.EchoPassword
			m.input.EchoCharacter = '•'
		} else {
			m.input.SetValue(m.getActualValue(row))
		}

		return m, textinput.Blink
	}

	return m, nil
}

func (m ConfigTUI) getActualValue(row configRow) string {
	if row.aiIndex == -1 {
		switch row.key {
		case "max_emails":
			return fmt.Sprintf("%d", m.cfg.MaxEmails)
		case "default_label":
			return m.cfg.DefaultLabel
		case "theme":
			return m.cfg.Theme
		}
	} else if row.aiIndex >= 0 && row.aiIndex < len(m.cfg.AIAccounts) {
		acc := m.cfg.AIAccounts[row.aiIndex]
		switch row.key {
		case "name":
			return acc.Name
		case "base_url":
			return acc.BaseURL
		case "model":
			return acc.Model
		case "api_key":
			return acc.APIKey
		}
	}
	return ""
}

func (m ConfigTUI) saveField() (tea.Model, tea.Cmd) {
	row := m.rows[m.cursor]
	value := m.input.Value()

	if row.aiIndex == -1 {
		switch row.key {
		case "max_emails":
			var n int
			fmt.Sscanf(value, "%d", &n)
			if n < 1 {
				n = 50
			}
			m.cfg.MaxEmails = n
		case "default_label":
			m.cfg.DefaultLabel = value
		case "theme":
			m.cfg.Theme = value
		}
	} else if row.aiIndex >= 0 && row.aiIndex < len(m.cfg.AIAccounts) {
		switch row.key {
		case "name":
			m.cfg.AIAccounts[row.aiIndex].Name = value
		case "base_url":
			m.cfg.AIAccounts[row.aiIndex].BaseURL = value
		case "model":
			m.cfg.AIAccounts[row.aiIndex].Model = value
		case "api_key":
			if value != "" {
				m.cfg.AIAccounts[row.aiIndex].APIKey = value
			}
		}
	}

	m.dirty = true
	m.editing = false
	m.buildRows()

	return m, nil
}

func (m ConfigTUI) View() string {
	var b strings.Builder

	title := "⚙  maily config"
	if m.editMode {
		title += cfgMutedStyle.Render(" [EDIT]")
	}
	if m.dirty {
		title += cfgDangerStyle.Render(" *")
	}
	b.WriteString(cfgTitleStyle.Render(title))
	b.WriteString("\n\n")

	for i, row := range m.rows {
		b.WriteString(m.renderRow(i, row))
		b.WriteString("\n")
	}

	if m.confirm {
		b.WriteString("\n")
		b.WriteString(cfgDangerStyle.Render("Unsaved changes! [s]ave / [d]iscard / [c]ancel"))
	}

	b.WriteString("\n")
	if m.editing {
		b.WriteString(cfgHintStyle.Render("enter confirm • esc cancel"))
	} else if m.editMode {
		b.WriteString(cfgHintStyle.Render("↑↓ navigate • enter edit • esc back"))
	} else {
		b.WriteString(cfgHintStyle.Render("↑↓ navigate • e edit • q quit"))
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m ConfigTUI) renderRow(idx int, row configRow) string {
	selected := idx == m.cursor

	// Action rows (only in edit mode)
	if row.isAction {
		text := row.label
		if row.key == "delete" {
			text = cfgDangerStyle.Render(row.label + " " + row.value)
		} else if row.key == "add" {
			text = cfgSuccessStyle.Render(row.label)
		}

		if selected {
			if row.key == "save" || row.key == "cancel" {
				return cfgSelectedStyle.Render(" " + row.label + " ")
			}
			return "▸ " + text
		}
		return "  " + text
	}

	// Config rows
	label := cfgLabelStyle.Render(row.label)
	value := cfgValueStyle.Render(row.value)
	if row.value == "" {
		value = cfgMutedStyle.Render("(empty)")
	}

	if m.editing && selected {
		return "▸ " + label + m.input.View()
	}

	if selected {
		return "▸ " + label + value
	}

	return "  " + label + value
}

func RunConfigTUI() error {
	p := tea.NewProgram(NewConfigTUI())
	_, err := p.Run()
	return err
}
