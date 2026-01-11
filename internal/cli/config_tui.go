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
			Foreground(purple)

	sectionStyle = lipgloss.NewStyle().
			Foreground(purpleLight).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(purpleLight)

	valueStyle = lipgloss.NewStyle().
			Foreground(white)

	emptyStyle = lipgloss.NewStyle().
			Foreground(gray).
			Italic(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(white).
			Background(purple).
			Bold(true)

	dimSelectedStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(grayDark)

	cursorStyle = lipgloss.NewStyle().
			Foreground(purple).
			Bold(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(gray)

	errorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	buttonStyle = lipgloss.NewStyle().
			Foreground(white).
			Background(grayDark).
			Padding(0, 2)

	buttonSelectedStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(purple).
				Bold(true).
				Padding(0, 2)

	buttonDangerStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(red).
				Padding(0, 2)

	buttonSuccessStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(green).
				Padding(0, 2)

	paneBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(grayDark)

	paneBorderActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(purple)

	fieldLabelStyle = lipgloss.NewStyle().
			Foreground(purpleLight).
			Bold(true).
			MarginBottom(1)

	fieldDescStyle = lipgloss.NewStyle().
			Foreground(gray).
			Italic(true)

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(0, 1)
)

type rowType int

const (
	rowTypeField rowType = iota
	rowTypeAction
	rowTypeSeparator
)

type pane int

const (
	leftPane pane = iota
	rightPane
)

type configRow struct {
	key         string
	label       string
	value       string
	aiIndex     int
	editable    bool
	isSecret    bool
	rowType     rowType
	description string
}

type rightPaneButton int

const (
	buttonSave rightPaneButton = iota
	buttonDiscard
)

type ConfigTUI struct {
	cfg              config.Config
	rows             []configRow
	cursor           int
	activePane       pane
	editing          bool
	input            textinput.Model
	dirty            bool
	confirm          bool
	loadErr          error
	saveErr          error
	fieldErr         string
	width            int
	height           int
	rightPaneButton rightPaneButton
}

func NewConfigTUI() ConfigTUI {
	cfg, err := config.Load()
	m := ConfigTUI{
		cfg:        cfg,
		loadErr:    err,
		width:      80,
		height:     24,
		activePane: leftPane,
	}
	m.buildRows()
	return m
}

func (m *ConfigTUI) buildRows() {
	m.rows = []configRow{
		// General section header
		{key: "separator", label: "General", rowType: rowTypeSeparator},
		// General settings
		{key: "max_emails", label: "Max Emails", value: fmt.Sprintf("%d", m.cfg.MaxEmails), aiIndex: -1, editable: true, rowType: rowTypeField, description: "Maximum number of emails to load per folder"},
		{key: "default_label", label: "Default Label", value: m.cfg.DefaultLabel, aiIndex: -1, editable: true, rowType: rowTypeField, description: "The folder/label to show on startup (e.g., INBOX)"},
		{key: "theme", label: "Theme", value: m.cfg.Theme, aiIndex: -1, editable: true, rowType: rowTypeField, description: "Color theme (dark, light)"},
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
			configRow{key: "name", label: "Name", value: acc.Name, aiIndex: i, editable: true, rowType: rowTypeField, description: "Display name for this AI provider"},
			configRow{key: "base_url", label: "Base URL", value: acc.BaseURL, aiIndex: i, editable: true, rowType: rowTypeField, description: "API endpoint URL"},
			configRow{key: "model", label: "Model", value: acc.Model, aiIndex: i, editable: true, rowType: rowTypeField, description: "Model identifier (e.g., gpt-4, claude-3)"},
			configRow{key: "api_key", label: "API Key", value: maskedKey, aiIndex: i, editable: true, isSecret: true, rowType: rowTypeField, description: "Your API key (stored securely)"},
			configRow{key: "delete", label: "Delete Account", aiIndex: i, rowType: rowTypeAction, description: "Remove this AI account"},
		)
	}

	// Actions
	m.rows = append(m.rows,
		configRow{key: "separator", label: "Actions", rowType: rowTypeSeparator},
		configRow{key: "add", label: "Add AI Account", rowType: rowTypeAction, description: "Add a new AI provider account"},
	)
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
		return m.updateNavigation(msg)
	}
	return m, nil
}

func (m ConfigTUI) updateNavigation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		// Switch panes
		if m.activePane == leftPane {
			m.activePane = rightPane
		} else {
			m.activePane = leftPane
		}
		return m, nil

	case "up", "k":
		if m.activePane == leftPane {
			m.moveCursor(-1)
		} else {
			// Navigate between save/discard in right pane
			if m.rightPaneButton == buttonDiscard {
				m.rightPaneButton = buttonSave
			}
		}
		return m, nil

	case "down", "j":
		if m.activePane == leftPane {
			m.moveCursor(1)
		} else {
			// Navigate between save/discard in right pane
			if m.rightPaneButton == buttonSave {
				m.rightPaneButton = buttonDiscard
			}
		}
		return m, nil

	case "enter", " ":
		return m.handleSelect()

	case "q", "esc", "ctrl+c":
		if m.dirty {
			m.confirm = true
			return m, nil
		}
		return m, tea.Quit
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
		m.activePane = leftPane
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
		m.confirm = false
		return m, tea.Quit
	case "n", "d":
		m.dirty = false
		m.confirm = false
		return m, tea.Quit
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
	// Handle right pane buttons
	if m.activePane == rightPane {
		switch m.rightPaneButton {
		case buttonSave:
			if err := m.cfg.Save(); err != nil {
				m.saveErr = err
				return m, nil
			}
			m.saveErr = nil
			m.dirty = false
			return m, tea.Quit
		case buttonDiscard:
			if m.dirty {
				m.confirm = true
				return m, nil
			}
			return m, tea.Quit
		}
		return m, nil
	}

	// Handle left pane selections
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
		m.activePane = rightPane
		m.fieldErr = ""
		m.input = textinput.New()
		m.input.Focus()
		m.input.CharLimit = 200
		m.input.Width = 30
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
	m.activePane = leftPane
	m.buildRows()

	return m, nil
}

func (m ConfigTUI) View() string {
	// Calculate pane widths (40% left, 60% right)
	totalWidth := max(m.width, 60)
	leftWidth := totalWidth * 4 / 10
	rightWidth := totalWidth - leftWidth - 4 // Account for borders and separator

	contentHeight := m.height - 6 // Account for title and footer

	// Render panes
	leftPaneView := m.renderLeftPane(leftWidth, contentHeight)
	rightPaneView := m.renderRightPane(rightWidth, contentHeight)

	// Join panes horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPaneView, rightPaneView)

	// Title
	title := titleStyle.Render("⚙  Maily Configuration")
	if m.dirty {
		title += errorStyle.Render(" (unsaved)")
	}

	// Errors
	var errLine string
	if m.loadErr != nil {
		errLine = errorStyle.Render("⚠ Error loading config: "+m.loadErr.Error()) + "\n"
	}
	if m.saveErr != nil {
		errLine += errorStyle.Render("⚠ Error saving config: "+m.saveErr.Error()) + "\n"
	}

	// Confirm dialog overlay
	if m.confirm {
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
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
	}

	// Footer hints
	var hint string
	if m.editing {
		hint = hintStyle.Render("Enter save · Esc cancel")
	} else {
		hint = hintStyle.Render("↑↓ navigate · Tab switch pane · Enter select · q quit")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		errLine,
		panels,
		hint,
	)
}

func (m ConfigTUI) renderLeftPane(width, height int) string {
	var content strings.Builder

	for i, row := range m.rows {
		selected := i == m.cursor
		isActive := m.activePane == leftPane

		line := m.renderLeftRow(row, selected, isActive)
		content.WriteString(line)
		content.WriteString("\n")
	}

	// Apply pane styling
	style := paneBorderStyle.
		Width(width).
		Height(height).
		Padding(1, 1)

	if m.activePane == leftPane {
		style = paneBorderActiveStyle.
			Width(width).
			Height(height).
			Padding(1, 1)
	}

	return style.Render(content.String())
}

func (m ConfigTUI) renderLeftRow(row configRow, selected, isActive bool) string {
	// Separator
	if row.rowType == rowTypeSeparator {
		return sectionStyle.Render("── " + row.label + " ──")
	}

	// Build the row content
	var prefix string
	if selected {
		if isActive {
			prefix = cursorStyle.Render("▸ ")
		} else {
			prefix = "▸ "
		}
	} else {
		prefix = "  "
	}

	// Action buttons
	if row.rowType == rowTypeAction {
		var label string
		switch row.key {
		case "delete":
			label = "✕ " + row.label
		case "add":
			label = "+ " + row.label
		default:
			label = row.label
		}

		if selected && isActive {
			return prefix + selectedStyle.Render(" "+label+" ")
		} else if selected {
			return prefix + dimSelectedStyle.Render(" "+label+" ")
		}
		return prefix + label
	}

	// Field rows
	label := labelStyle.Width(14).Render(row.label)
	sanitized := sanitizeValue(row.value)
	var value string
	if sanitized == "" {
		value = emptyStyle.Render("(not set)")
	} else {
		// Truncate long values
		maxValLen := 12
		if len(sanitized) > maxValLen {
			sanitized = sanitized[:maxValLen-2] + ".."
		}
		value = valueStyle.Render(sanitized)
	}

	if selected && isActive {
		return prefix + label + selectedStyle.Render(" "+sanitized+" ")
	} else if selected {
		return prefix + label + dimSelectedStyle.Render(" "+sanitized+" ")
	}
	return prefix + label + value
}

func (m ConfigTUI) renderRightPane(width, height int) string {
	var content strings.Builder

	if m.cursor < len(m.rows) {
		row := m.rows[m.cursor]

		if row.rowType != rowTypeSeparator {
			// Field label
			content.WriteString(fieldLabelStyle.Render(row.label))
			content.WriteString("\n")
			content.WriteString(strings.Repeat("─", min(width-4, 30)))
			content.WriteString("\n\n")

			// Description
			if row.description != "" {
				content.WriteString(fieldDescStyle.Render(row.description))
				content.WriteString("\n\n")
			}

			switch row.rowType {
			case rowTypeField:
				// Current value
				currentVal := m.getActualValue(row)
				if row.isSecret && currentVal != "" {
					if len(currentVal) > 8 {
						currentVal = currentVal[:4] + "••••" + currentVal[len(currentVal)-4:]
					} else {
						currentVal = "••••••••"
					}
				}
				if currentVal == "" {
					currentVal = "(not set)"
				}
				content.WriteString(lipgloss.NewStyle().Foreground(gray).Render("Current: "))
				content.WriteString(valueStyle.Render(currentVal))
				content.WriteString("\n\n")

				// Input box (when editing)
				if m.editing {
					inputView := inputBoxStyle.Width(min(width-6, 35)).Render(m.input.View())
					content.WriteString(inputView)
					content.WriteString("\n")

					if m.fieldErr != "" {
						content.WriteString(errorStyle.Render("⚠ " + m.fieldErr))
						content.WriteString("\n")
					}

					content.WriteString("\n")
					content.WriteString(hintStyle.Render("Enter to save · Esc to cancel"))
				} else {
					content.WriteString(hintStyle.Render("Press Enter to edit"))
				}
			case rowTypeAction:
				// Action description
				switch row.key {
				case "delete":
					content.WriteString(errorStyle.Render("This will remove the AI account."))
					content.WriteString("\n\n")
					content.WriteString(hintStyle.Render("Press Enter to confirm"))
				case "add":
					content.WriteString(valueStyle.Render("Add a new AI provider configuration."))
					content.WriteString("\n\n")
					content.WriteString(hintStyle.Render("Press Enter to add"))
				}
			}
		}
	}

	// Separator line before buttons
	content.WriteString("\n\n")
	content.WriteString(lipgloss.NewStyle().Foreground(grayDark).Render(strings.Repeat("─", min(width-4, 30))))
	content.WriteString("\n\n")

	// Save/Discard buttons
	isActive := m.activePane == rightPane && !m.editing

	saveBtn := buttonSuccessStyle.Render(" ✓ Save ")
	discardBtn := buttonStyle.Render(" ✕ Quit ")

	if isActive {
		if m.rightPaneButton == buttonSave {
			saveBtn = buttonSelectedStyle.Render(" ✓ Save ")
		} else {
			discardBtn = buttonSelectedStyle.Render(" ✕ Quit ")
		}
	}

	content.WriteString(saveBtn + "  " + discardBtn)

	// Apply pane styling
	style := paneBorderStyle.
		Width(width).
		Height(height).
		Padding(1, 2)

	if m.activePane == rightPane {
		style = paneBorderActiveStyle.
			Width(width).
			Height(height).
			Padding(1, 2)
	}

	return style.Render(content.String())
}

func RunConfigTUI() error {
	p := tea.NewProgram(NewConfigTUI(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
