package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"maily/config"
)

// ansiRegex matches ANSI escape sequences
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// sanitizeValue strips ANSI escape sequences and control characters from config values
func sanitizeValue(s string) string {
	s = ansiRegex.ReplaceAllString(s, "")
	var b strings.Builder
	for _, r := range s {
		if r >= 32 || r == '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Colors
var (
	purple      = lipgloss.Color("#7C3AED")
	purpleLight = lipgloss.Color("#A78BFA")
	green       = lipgloss.Color("#10B981")
	red         = lipgloss.Color("#EF4444")
	gray        = lipgloss.Color("#6B7280")
	grayDark    = lipgloss.Color("#374151")
	white       = lipgloss.Color("#F9FAFB")
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(purple).
			MarginBottom(1)

	sectionStyle = lipgloss.NewStyle().
			Foreground(purpleLight).
			Bold(true).
			MarginTop(1).
			MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Foreground(purpleLight).
			Width(22)

	valueStyle = lipgloss.NewStyle().
			Foreground(white)

	emptyStyle = lipgloss.NewStyle().
			Foreground(gray).
			Italic(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(white).
			Background(purple).
			Bold(true).
			Padding(0, 1)

	cursorStyle = lipgloss.NewStyle().
			Foreground(purple).
			Bold(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(gray).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	buttonStyle = lipgloss.NewStyle().
			Foreground(white).
			Background(grayDark).
			Padding(0, 2).
			MarginRight(1)

	buttonSelectedStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(purple).
				Bold(true).
				Padding(0, 2).
				MarginRight(1)

	buttonDangerStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(red).
				Padding(0, 2).
				MarginRight(1)

	buttonSuccessStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(green).
				Padding(0, 2).
				MarginRight(1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(grayDark).
			Padding(1, 2)

	editBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(1, 2)
)

type rowType int

const (
	rowTypeField rowType = iota
	rowTypeAction
	rowTypeSeparator
)

type configRow struct {
	key      string
	label    string
	value    string
	aiIndex  int
	editable bool
	isSecret bool
	rowType  rowType
}

type ConfigTUI struct {
	cfg       config.Config
	rows      []configRow
	cursor    int
	editMode  bool
	editing   bool
	input     textinput.Model
	dirty     bool
	confirm   bool
	loadErr   error
	saveErr   error
	fieldErr  string
	width     int
	height    int
}

func NewConfigTUI() ConfigTUI {
	cfg, err := config.Load()
	m := ConfigTUI{
		cfg:     cfg,
		loadErr: err,
		width:   80,
		height:  24,
	}
	m.buildRows()
	return m
}

func (m *ConfigTUI) buildRows() {
	m.rows = []configRow{
		// General settings
		{key: "max_emails", label: "Max Emails", value: fmt.Sprintf("%d", m.cfg.MaxEmails), aiIndex: -1, editable: true, rowType: rowTypeField},
		{key: "default_label", label: "Default Label", value: m.cfg.DefaultLabel, aiIndex: -1, editable: true, rowType: rowTypeField},
		{key: "theme", label: "Theme", value: m.cfg.Theme, aiIndex: -1, editable: true, rowType: rowTypeField},
	}

	// AI Accounts
	for i, acc := range m.cfg.AIAccounts {
		maskedKey := "••••••••"
		if len(acc.APIKey) > 8 {
			maskedKey = acc.APIKey[:4] + "••••" + acc.APIKey[len(acc.APIKey)-4:]
		} else if acc.APIKey == "" {
			maskedKey = ""
		}

		prefix := fmt.Sprintf("AI Account %d", i+1)
		m.rows = append(m.rows,
			configRow{key: "separator", label: prefix, rowType: rowTypeSeparator},
			configRow{key: "name", label: "Name", value: acc.Name, aiIndex: i, editable: true, rowType: rowTypeField},
			configRow{key: "base_url", label: "Base URL", value: acc.BaseURL, aiIndex: i, editable: true, rowType: rowTypeField},
			configRow{key: "model", label: "Model", value: acc.Model, aiIndex: i, editable: true, rowType: rowTypeField},
			configRow{key: "api_key", label: "API Key", value: maskedKey, aiIndex: i, editable: true, isSecret: true, rowType: rowTypeField},
			configRow{key: "delete", label: "Delete Account", aiIndex: i, rowType: rowTypeAction},
		)
	}

	// Actions (only in edit mode)
	if m.editMode {
		m.rows = append(m.rows,
			configRow{key: "separator", label: "Actions", rowType: rowTypeSeparator},
			configRow{key: "add", label: "Add AI Account", rowType: rowTypeAction},
			configRow{key: "save", label: "Save Changes", rowType: rowTypeAction},
			configRow{key: "cancel", label: "Cancel", rowType: rowTypeAction},
		)
	}
}

func (m ConfigTUI) Init() tea.Cmd {
	return nil
}

func (m ConfigTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

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
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "e":
		if m.loadErr != nil {
			return m, nil
		}
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
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "enter", " ":
		return m.handleSelect()
	case "q", "esc", "ctrl+c":
		if m.dirty {
			m.confirm = true
			return m, nil
		}
		m.editMode = false
		m.buildRows()
		m.clampCursor()
	}
	return m, nil
}

func (m ConfigTUI) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.saveField()
	case "esc":
		m.editing = false
		m.fieldErr = ""
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m ConfigTUI) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "s":
		if err := m.cfg.Save(); err != nil {
			m.saveErr = err
			m.confirm = false
			return m, nil
		}
		m.saveErr = nil
		m.dirty = false
		m.editMode = false
		m.confirm = false
		m.buildRows()
		m.clampCursor()
	case "n", "d":
		m.dirty = false
		m.editMode = false
		m.confirm = false
		m.saveErr = nil
		m.cfg, m.loadErr = config.Load()
		m.buildRows()
		m.clampCursor()
	case "esc", "c":
		m.confirm = false
	}
	return m, nil
}

func (m *ConfigTUI) moveCursor(delta int) {
	newCursor := m.cursor + delta
	for newCursor >= 0 && newCursor < len(m.rows) {
		if m.rows[newCursor].rowType != rowTypeSeparator {
			m.cursor = newCursor
			return
		}
		newCursor += delta
	}
}

func (m *ConfigTUI) clampCursor() {
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	// Skip separators
	for m.cursor < len(m.rows) && m.rows[m.cursor].rowType == rowTypeSeparator {
		m.cursor++
	}
}

func (m ConfigTUI) handleSelect() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.rows) {
		return m, nil
	}
	row := m.rows[m.cursor]

	switch row.key {
	case "add":
		m.cfg.AIAccounts = append(m.cfg.AIAccounts, config.AIAccount{})
		m.dirty = true
		m.buildRows()
		return m, nil

	case "save":
		if err := m.cfg.Save(); err != nil {
			m.saveErr = err
			return m, nil
		}
		m.saveErr = nil
		m.dirty = false
		m.editMode = false
		m.buildRows()
		m.clampCursor()
		return m, nil

	case "cancel":
		if m.dirty {
			m.confirm = true
			return m, nil
		}
		m.editMode = false
		m.buildRows()
		m.clampCursor()
		return m, nil

	case "delete":
		if row.aiIndex >= 0 && row.aiIndex < len(m.cfg.AIAccounts) {
			m.cfg.AIAccounts = append(m.cfg.AIAccounts[:row.aiIndex], m.cfg.AIAccounts[row.aiIndex+1:]...)
			m.dirty = true
			m.buildRows()
			m.clampCursor()
		}
		return m, nil
	}

	if row.editable {
		m.editing = true
		m.fieldErr = ""
		m.input = textinput.New()
		m.input.Focus()
		m.input.CharLimit = 200
		m.input.Width = 40
		m.input.Prompt = ""

		if row.isSecret {
			m.input.Placeholder = "Enter new value..."
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
	changed := false
	m.fieldErr = ""

	if row.aiIndex == -1 {
		switch row.key {
		case "max_emails":
			var newVal int
			n, err := fmt.Sscanf(value, "%d", &newVal)
			if err != nil || n != 1 || newVal < 1 {
				m.fieldErr = "Must be a positive integer"
				return m, nil
			}
			if newVal != m.cfg.MaxEmails {
				m.cfg.MaxEmails = newVal
				changed = true
			}
		case "default_label":
			if value != m.cfg.DefaultLabel {
				m.cfg.DefaultLabel = value
				changed = true
			}
		case "theme":
			if value != m.cfg.Theme {
				m.cfg.Theme = value
				changed = true
			}
		}
	} else if row.aiIndex >= 0 && row.aiIndex < len(m.cfg.AIAccounts) {
		acc := &m.cfg.AIAccounts[row.aiIndex]
		switch row.key {
		case "name":
			if value != acc.Name {
				acc.Name = value
				changed = true
			}
		case "base_url":
			if value != acc.BaseURL {
				acc.BaseURL = value
				changed = true
			}
		case "model":
			if value != acc.Model {
				acc.Model = value
				changed = true
			}
		case "api_key":
			if value != "" && value != acc.APIKey {
				acc.APIKey = value
				changed = true
			}
		}
	}

	if changed {
		m.dirty = true
	}
	m.editing = false
	m.buildRows()

	return m, nil
}

func (m ConfigTUI) View() string {
	var content strings.Builder

	// Title
	title := "⚙  Maily Configuration"
	if m.dirty {
		title += errorStyle.Render(" (unsaved)")
	}
	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n\n")

	// Errors
	if m.loadErr != nil {
		content.WriteString(errorStyle.Render("⚠ Error loading config: " + m.loadErr.Error()))
		content.WriteString("\n\n")
	}
	if m.saveErr != nil {
		content.WriteString(errorStyle.Render("⚠ Error saving config: " + m.saveErr.Error()))
		content.WriteString("\n\n")
	}

	// Mode indicator
	if m.editMode {
		content.WriteString(sectionStyle.Render("━━━ Edit Mode ━━━"))
		content.WriteString("\n\n")
	}

	// Rows
	for i, row := range m.rows {
		content.WriteString(m.renderRow(i, row))
		content.WriteString("\n")
	}

	// Confirm dialog
	if m.confirm {
		content.WriteString("\n")
		dialog := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(red).
			Padding(1, 2).
			Render(
				errorStyle.Render("Unsaved changes!") + "\n\n" +
					buttonSuccessStyle.Render(" S  Save ") + "  " +
					buttonDangerStyle.Render(" D  Discard ") + "  " +
					buttonStyle.Render(" C  Cancel "),
			)
		content.WriteString(dialog)
	}

	// Hints
	content.WriteString("\n")
	if m.editing {
		content.WriteString(hintStyle.Render("Enter to save • Esc to cancel"))
	} else if m.editMode {
		content.WriteString(hintStyle.Render("↑↓ Navigate • Enter/Space to edit • Esc to exit"))
	} else if m.loadErr != nil {
		content.WriteString(hintStyle.Render("↑↓ Navigate • Q to quit (editing disabled)"))
	} else {
		content.WriteString(hintStyle.Render("↑↓ Navigate • E to edit • Q to quit"))
	}

	// Wrap in box
	boxStyleToUse := boxStyle
	if m.editMode {
		boxStyleToUse = editBoxStyle
	}

	return boxStyleToUse.Render(content.String())
}

func (m ConfigTUI) renderRow(idx int, row configRow) string {
	selected := idx == m.cursor

	// Separator
	if row.rowType == rowTypeSeparator {
		return "\n" + sectionStyle.Render("─── "+row.label+" ───")
	}

	// Action buttons
	if row.rowType == rowTypeAction {
		var btn string
		switch row.key {
		case "add":
			if selected {
				btn = buttonSelectedStyle.Render(" + " + row.label + " ")
			} else {
				btn = buttonSuccessStyle.Render(" + " + row.label + " ")
			}
		case "delete":
			if selected {
				btn = buttonSelectedStyle.Render(" ✕ " + row.label + " ")
			} else {
				btn = buttonDangerStyle.Render(" ✕ " + row.label + " ")
			}
		case "save":
			if selected {
				btn = buttonSelectedStyle.Render(" ✓ " + row.label + " ")
			} else {
				btn = buttonSuccessStyle.Render(" ✓ " + row.label + " ")
			}
		case "cancel":
			if selected {
				btn = buttonSelectedStyle.Render(" ✕ " + row.label + " ")
			} else {
				btn = buttonStyle.Render(" ✕ " + row.label + " ")
			}
		default:
			if selected {
				btn = buttonSelectedStyle.Render(" " + row.label + " ")
			} else {
				btn = buttonStyle.Render(" " + row.label + " ")
			}
		}

		if selected {
			return cursorStyle.Render("▸ ") + btn
		}
		return "  " + btn
	}

	// Field rows
	label := labelStyle.Render(row.label)
	sanitized := sanitizeValue(row.value)
	value := valueStyle.Render(sanitized)
	if sanitized == "" {
		value = emptyStyle.Render("(not set)")
	}

	// Currently editing this field
	if m.editing && selected {
		inputView := m.input.View()
		line := cursorStyle.Render("▸ ") + label + inputView
		if m.fieldErr != "" {
			line += "  " + errorStyle.Render("⚠ " + m.fieldErr)
		}
		return line
	}

	// Selected but not editing
	if selected {
		return cursorStyle.Render("▸ ") + label + selectedStyle.Render(" "+sanitized+" ")
	}

	// Normal row
	return "  " + label + value
}

func RunConfigTUI() error {
	p := tea.NewProgram(NewConfigTUI(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
