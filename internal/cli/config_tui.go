package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"maily/config"
	"maily/internal/i18n"
)

// Styles
var (
	cfgPurple = lipgloss.Color("#7C3AED")
	cfgGray   = lipgloss.Color("#6B7280")
	cfgWhite  = lipgloss.Color("#F9FAFB")
	cfgRed    = lipgloss.Color("#EF4444")

	cfgTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cfgPurple).
			MarginBottom(1)

	cfgSectionStyle = lipgloss.NewStyle().
			Foreground(cfgPurple).
			Bold(true)

	cfgLabelStyle = lipgloss.NewStyle().
			Foreground(cfgGray).
			Width(14)

	cfgValueStyle = lipgloss.NewStyle().
			Foreground(cfgWhite)

	cfgSelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cfgWhite).
			Background(cfgPurple)

	cfgHintStyle = lipgloss.NewStyle().
			Foreground(cfgGray)

	cfgErrorStyle = lipgloss.NewStyle().
			Foreground(cfgRed)

	cfgButtonStyle = lipgloss.NewStyle().
			Foreground(cfgWhite).
			Background(lipgloss.Color("#374151")).
			Padding(0, 1)

	cfgButtonSelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cfgWhite).
			Background(cfgPurple).
			Padding(0, 1)

	cfgDialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cfgPurple).
			Padding(1, 2)
)

type rowKind int

const (
	rowField rowKind = iota
	rowSection
	rowAction
)

type row struct {
	kind        rowKind
	key         string // field key for editing
	label       string
	value       string
	providerIdx int  // -1 for general settings
	isSecret    bool
}

type quitOption int

const (
	quitOptionSave quitOption = iota
	quitOptionDiscard
	quitOptionCancel
)

type ConfigTUI struct {
	cfg       config.Config
	rows      []row
	cursor    int
	editing   bool
	input     textinput.Model
	dirty     bool
	err       error
	width     int
	height    int

	// Provider dialog
	showProviderDialog bool
	providerType       config.AIProviderType
	providerInputs     []textinput.Model // name, model, base_url, api_key
	providerFocus      int               // which input is focused
	editingProviderIdx int               // -1 for new, >= 0 for editing existing

	// Quit confirmation
	showQuitConfirm bool
	quitOption      quitOption

	// Language picker
	showLanguagePicker bool
	languageCursor     int
}

func NewConfigTUI() ConfigTUI {
	cfg, err := config.Load()
	m := ConfigTUI{
		cfg:    cfg,
		err:    err,
		width:  80,
		height: 24,
	}
	m.buildRows()
	return m
}

func (m *ConfigTUI) buildRows() {
	// Get language display name
	langDisplay := i18n.DisplayName(m.cfg.Language)
	if m.cfg.Language == "" {
		langDisplay = "Auto (" + i18n.DisplayName(i18n.CurrentLanguage()) + ")"
	}

	m.rows = []row{
		{kind: rowSection, label: "General"},
		{kind: rowField, key: "max_emails", label: "Max Emails", value: fmt.Sprintf("%d", m.cfg.MaxEmails), providerIdx: -1},
		{kind: rowField, key: "default_label", label: "Default Label", value: m.cfg.DefaultLabel, providerIdx: -1},
		{kind: rowField, key: "theme", label: "Theme", value: m.cfg.Theme, providerIdx: -1},
		{kind: rowAction, key: "language", label: "Language", value: langDisplay, providerIdx: -1},
	}

	// AI Providers
	if len(m.cfg.AIProviders) > 0 {
		m.rows = append(m.rows, row{kind: rowSection, label: "AI Providers"})
		for i, p := range m.cfg.AIProviders {
			label := fmt.Sprintf("%s/%s", p.Name, p.Model)
			if p.Name == "" {
				label = "(not configured)"
			}
			m.rows = append(m.rows, row{kind: rowAction, key: "edit_provider", label: label, value: string(p.Type), providerIdx: i})
		}
	}

	// Actions
	m.rows = append(m.rows, row{kind: rowSection, label: "Actions"})
	m.rows = append(m.rows, row{kind: rowAction, key: "add_cli", label: "Add CLI Provider (claude, codex, gemini...)"})
	m.rows = append(m.rows, row{kind: rowAction, key: "add_api", label: "Add API Provider (OpenAI, etc.)"})
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
		if m.showQuitConfirm {
			return m.updateQuitConfirm(msg)
		}
		if m.showLanguagePicker {
			return m.updateLanguagePicker(msg)
		}
		if m.showProviderDialog {
			return m.updateProviderDialog(msg)
		}
		if m.editing {
			return m.updateEditing(msg)
		}
		return m.updateNav(msg)
	}
	return m, nil
}

func (m ConfigTUI) updateNav(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "enter", " ":
		return m.handleSelect()
	case "s":
		// Save
		if err := m.cfg.Save(); err != nil {
			m.err = err
			return m, nil
		}
		m.dirty = false
		m.err = nil
	case "q", "esc":
		if m.dirty {
			m.showQuitConfirm = true
			m.quitOption = quitOptionSave // default to save
			return m, nil
		}
		return m, tea.Quit
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m ConfigTUI) updateQuitConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		if m.quitOption > quitOptionSave {
			m.quitOption--
		}
	case "right", "l":
		if m.quitOption < quitOptionCancel {
			m.quitOption++
		}
	case "enter":
		switch m.quitOption {
		case quitOptionSave:
			if err := m.cfg.Save(); err != nil {
				m.err = err
				m.showQuitConfirm = false
				return m, nil
			}
			return m, tea.Quit
		case quitOptionDiscard:
			return m, tea.Quit
		case quitOptionCancel:
			m.showQuitConfirm = false
		}
	case "esc":
		m.showQuitConfirm = false
	case "s":
		// Quick save and quit
		if err := m.cfg.Save(); err != nil {
			m.err = err
			m.showQuitConfirm = false
			return m, nil
		}
		return m, tea.Quit
	case "d":
		// Quick discard
		return m, tea.Quit
	case "c":
		// Quick cancel
		m.showQuitConfirm = false
	}
	return m, nil
}

func (m ConfigTUI) updateLanguagePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Total options = 1 (Auto) + len(SupportedLanguages)
	total := 1 + len(i18n.SupportedLanguages)

	switch msg.String() {
	case "up", "k":
		if m.languageCursor > 0 {
			m.languageCursor--
		}
	case "down", "j":
		if m.languageCursor < total-1 {
			m.languageCursor++
		}
	case "enter":
		var newLang string
		if m.languageCursor == 0 {
			newLang = "" // Auto-detect
		} else {
			newLang = i18n.SupportedLanguages[m.languageCursor-1]
		}
		if newLang != m.cfg.Language {
			m.cfg.Language = newLang
			m.dirty = true
			m.buildRows()
		}
		m.showLanguagePicker = false
	case "esc":
		m.showLanguagePicker = false
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

func (m *ConfigTUI) moveCursor(delta int) {
	newCursor := m.cursor + delta
	for newCursor >= 0 && newCursor < len(m.rows) {
		if m.rows[newCursor].kind != rowSection {
			m.cursor = newCursor
			return
		}
		newCursor += delta
	}
}

func (m ConfigTUI) handleSelect() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.rows) {
		return m, nil
	}
	r := m.rows[m.cursor]

	switch r.kind {
	case rowField:
		// Start editing
		m.editing = true
		m.input = textinput.New()
		m.input.Focus()
		m.input.CharLimit = 200
		m.input.Width = 40
		m.input.Prompt = ""

		if r.isSecret {
			m.input.Placeholder = "Enter new value..."
			m.input.EchoMode = textinput.EchoPassword
			m.input.EchoCharacter = '•'
		} else {
			m.input.SetValue(m.getFieldValue(r))
		}
		return m, textinput.Blink

	case rowAction:
		switch r.key {
		case "language":
			m.showLanguagePicker = true
			// Find current language in list
			m.languageCursor = 0 // Default to first (Auto)
			for i, lang := range i18n.SupportedLanguages {
				if lang == m.cfg.Language {
					m.languageCursor = i + 1 // +1 because Auto is at index 0
					break
				}
			}
			return m, nil
		case "add_cli":
			m.openProviderDialog(config.AIProviderTypeCLI, -1)
			return m, textinput.Blink
		case "add_api":
			m.openProviderDialog(config.AIProviderTypeAPI, -1)
			return m, textinput.Blink
		case "edit_provider":
			if r.providerIdx >= 0 && r.providerIdx < len(m.cfg.AIProviders) {
				p := m.cfg.AIProviders[r.providerIdx]
				m.openProviderDialog(p.Type, r.providerIdx)
				return m, textinput.Blink
			}
		}
	}
	return m, nil
}

func (m *ConfigTUI) clampCursor() {
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	for m.cursor < len(m.rows) && m.rows[m.cursor].kind == rowSection {
		m.cursor++
	}
}

func (m *ConfigTUI) openProviderDialog(providerType config.AIProviderType, editIdx int) {
	m.showProviderDialog = true
	m.providerType = providerType
	m.editingProviderIdx = editIdx
	m.providerFocus = 0

	// Create inputs: name, model (and base_url, api_key for API type)
	numInputs := 2
	if providerType == config.AIProviderTypeAPI {
		numInputs = 4
	}

	m.providerInputs = make([]textinput.Model, numInputs)

	// Name input
	m.providerInputs[0] = textinput.New()
	m.providerInputs[0].Placeholder = "claude, codex, gemini..."
	m.providerInputs[0].Width = 30
	m.providerInputs[0].Prompt = ""
	m.providerInputs[0].Focus()

	// Model input
	m.providerInputs[1] = textinput.New()
	m.providerInputs[1].Placeholder = "haiku, o4-mini, gemini-2.5-flash..."
	m.providerInputs[1].Width = 30
	m.providerInputs[1].Prompt = ""

	if providerType == config.AIProviderTypeAPI {
		// Base URL input
		m.providerInputs[2] = textinput.New()
		m.providerInputs[2].Placeholder = "https://api.openai.com/v1"
		m.providerInputs[2].Width = 30
		m.providerInputs[2].Prompt = ""

		// API Key input
		m.providerInputs[3] = textinput.New()
		m.providerInputs[3].Placeholder = "sk-..."
		m.providerInputs[3].Width = 30
		m.providerInputs[3].Prompt = ""
		m.providerInputs[3].EchoMode = textinput.EchoPassword
		m.providerInputs[3].EchoCharacter = '•'
	}

	// Pre-fill if editing existing provider
	if editIdx >= 0 && editIdx < len(m.cfg.AIProviders) {
		p := m.cfg.AIProviders[editIdx]
		m.providerInputs[0].SetValue(p.Name)
		m.providerInputs[1].SetValue(p.Model)
		if providerType == config.AIProviderTypeAPI {
			m.providerInputs[2].SetValue(p.BaseURL)
			m.providerInputs[3].SetValue(p.APIKey)
		}
	}
}

func (m ConfigTUI) updateProviderDialog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		m.providerInputs[m.providerFocus].Blur()
		m.providerFocus = (m.providerFocus + 1) % len(m.providerInputs)
		m.providerInputs[m.providerFocus].Focus()
		return m, textinput.Blink
	case "shift+tab", "up":
		m.providerInputs[m.providerFocus].Blur()
		m.providerFocus--
		if m.providerFocus < 0 {
			m.providerFocus = len(m.providerInputs) - 1
		}
		m.providerInputs[m.providerFocus].Focus()
		return m, textinput.Blink
	case "enter":
		// Save provider
		name := m.providerInputs[0].Value()
		model := m.providerInputs[1].Value()
		if name == "" || model == "" {
			return m, nil // require name and model
		}

		p := config.AIProvider{
			Type:  m.providerType,
			Name:  name,
			Model: model,
		}
		if m.providerType == config.AIProviderTypeAPI {
			p.BaseURL = m.providerInputs[2].Value()
			p.APIKey = m.providerInputs[3].Value()
		}

		if m.editingProviderIdx >= 0 {
			m.cfg.AIProviders[m.editingProviderIdx] = p
		} else {
			m.cfg.AIProviders = append(m.cfg.AIProviders, p)
		}
		m.dirty = true
		m.showProviderDialog = false
		m.buildRows()
		return m, nil
	case "esc":
		m.showProviderDialog = false
		return m, nil
	case "d", "ctrl+d":
		// Delete provider (only when editing existing)
		if m.editingProviderIdx >= 0 && m.editingProviderIdx < len(m.cfg.AIProviders) {
			m.cfg.AIProviders = append(m.cfg.AIProviders[:m.editingProviderIdx], m.cfg.AIProviders[m.editingProviderIdx+1:]...)
			m.dirty = true
			m.showProviderDialog = false
			m.buildRows()
			m.clampCursor()
			return m, nil
		}
	}

	// Update focused input
	var cmd tea.Cmd
	m.providerInputs[m.providerFocus], cmd = m.providerInputs[m.providerFocus].Update(msg)
	return m, cmd
}

func (m ConfigTUI) getFieldValue(r row) string {
	if r.providerIdx == -1 {
		switch r.key {
		case "max_emails":
			return fmt.Sprintf("%d", m.cfg.MaxEmails)
		case "default_label":
			return m.cfg.DefaultLabel
		case "theme":
			return m.cfg.Theme
		}
	} else if r.providerIdx >= 0 && r.providerIdx < len(m.cfg.AIProviders) {
		p := m.cfg.AIProviders[r.providerIdx]
		switch r.key {
		case "type":
			return string(p.Type)
		case "name":
			return p.Name
		case "model":
			return p.Model
		case "base_url":
			return p.BaseURL
		case "api_key":
			return p.APIKey
		}
	}
	return ""
}

func (m ConfigTUI) saveField() (tea.Model, tea.Cmd) {
	r := m.rows[m.cursor]
	value := m.input.Value()
	changed := false

	if r.providerIdx == -1 {
		switch r.key {
		case "max_emails":
			var newVal int
			if _, err := fmt.Sscanf(value, "%d", &newVal); err == nil && newVal > 0 {
				if newVal != m.cfg.MaxEmails {
					m.cfg.MaxEmails = newVal
					changed = true
				}
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
	} else if r.providerIdx >= 0 && r.providerIdx < len(m.cfg.AIProviders) {
		p := &m.cfg.AIProviders[r.providerIdx]
		switch r.key {
		case "type":
			newType := config.AIProviderType(value)
			if newType != p.Type {
				p.Type = newType
				changed = true
			}
		case "name":
			if value != p.Name {
				p.Name = value
				changed = true
			}
		case "model":
			if value != p.Model {
				p.Model = value
				changed = true
			}
		case "base_url":
			if value != p.BaseURL {
				p.BaseURL = value
				changed = true
			}
		case "api_key":
			if value != "" && value != p.APIKey {
				p.APIKey = value
				changed = true
			}
		}
	}

	if changed {
		m.dirty = true
		m.buildRows()
	}
	m.editing = false
	return m, nil
}

func (m ConfigTUI) View() string {
	var b strings.Builder

	// Top padding
	b.WriteString("\n\n")

	// Left padding for all content
	pad := "   "

	// Title
	title := cfgTitleStyle.Render("Maily Configuration")
	if m.dirty {
		title += cfgErrorStyle.Render(" *")
	}
	b.WriteString(pad + title + "\n\n")

	// Error
	if m.err != nil {
		b.WriteString(pad + cfgErrorStyle.Render("Error: "+m.err.Error()) + "\n\n")
	}

	// Rows
	for i, r := range m.rows {
		selected := i == m.cursor

		switch r.kind {
		case rowSection:
			b.WriteString("\n" + pad + cfgSectionStyle.Render("─── "+r.label+" ───") + "\n")

		case rowField:
			label := cfgLabelStyle.Render(r.label)
			value := r.value
			if value == "" {
				value = cfgHintStyle.Render("(not set)")
			} else {
				value = cfgValueStyle.Render(value)
			}

			line := pad + "  " + label + " " + value
			if selected {
				line = pad + cfgSelectedStyle.Render(" ▸ " + r.label + ": " + r.value + " ")
			}
			b.WriteString(line + "\n")

		case rowAction:
			var line string
			if r.key == "language" {
				// Language row: show like a field with value
				label := cfgLabelStyle.Render(r.label)
				value := cfgValueStyle.Render(r.value)
				line = pad + "  " + label + " " + value
				if selected {
					line = pad + cfgSelectedStyle.Render(" ▸ " + r.label + ": " + r.value + " ")
				}
			} else if r.key == "edit_provider" {
				// Provider row: show type and name/model
				typeLabel := cfgHintStyle.Render("[" + r.value + "]")
				if selected {
					line = pad + cfgSelectedStyle.Render(" ▸ " + r.label + " ") + " " + typeLabel
				} else {
					line = pad + "  " + cfgValueStyle.Render(r.label) + " " + typeLabel
				}
			} else {
				// Add action
				line = pad + "  " + cfgHintStyle.Render("+ "+r.label)
				if selected {
					line = pad + cfgSelectedStyle.Render(" ▸ + " + r.label + " ")
				}
			}
			b.WriteString(line + "\n")
		}
	}

	// Footer
	b.WriteString("\n" + pad + cfgHintStyle.Render("↑↓ navigate · Enter edit · s save · q quit") + "\n")

	// Provider dialog overlay
	if m.showProviderDialog {
		var dialogContent strings.Builder

		title := "Add CLI Provider"
		if m.providerType == config.AIProviderTypeAPI {
			title = "Add API Provider"
		}
		if m.editingProviderIdx >= 0 {
			title = "Edit Provider"
		}
		dialogContent.WriteString(cfgSectionStyle.Render(title) + "\n\n")

		labels := []string{"Name", "Model"}
		if m.providerType == config.AIProviderTypeAPI {
			labels = []string{"Name", "Model", "Base URL", "API Key"}
		}

		for i, input := range m.providerInputs {
			label := cfgLabelStyle.Width(10).Render(labels[i])
			inputView := input.View()
			if i == m.providerFocus {
				dialogContent.WriteString(cfgSelectedStyle.Render("▸") + " " + label + inputView + "\n")
			} else {
				dialogContent.WriteString("  " + label + inputView + "\n")
			}
		}

		hints := "Tab next · Enter save · Esc cancel"
		if m.editingProviderIdx >= 0 {
			hints = "Tab next · Enter save · d delete · Esc cancel"
		}
		dialogContent.WriteString("\n" + cfgHintStyle.Render(hints))

		dialog := cfgDialogStyle.Render(dialogContent.String())
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
	}

	// Edit dialog overlay
	if m.editing {
		r := m.rows[m.cursor]
		dialog := cfgDialogStyle.Render(
			cfgSectionStyle.Render("Edit: "+r.label) + "\n\n" +
				m.input.View() + "\n\n" +
				cfgHintStyle.Render("Enter save · Esc cancel"),
		)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
	}

	// Quit confirmation dialog
	if m.showQuitConfirm {
		var saveBtn, discardBtn, cancelBtn string

		if m.quitOption == quitOptionSave {
			saveBtn = cfgButtonSelectedStyle.Render(" S  Save & Quit ")
		} else {
			saveBtn = cfgButtonStyle.Render(" S  Save & Quit ")
		}
		if m.quitOption == quitOptionDiscard {
			discardBtn = lipgloss.NewStyle().Foreground(cfgWhite).Background(cfgRed).Padding(0, 1).Render(" D  Discard ")
		} else {
			discardBtn = cfgButtonStyle.Render(" D  Discard ")
		}
		if m.quitOption == quitOptionCancel {
			cancelBtn = cfgButtonSelectedStyle.Render(" C  Cancel ")
		} else {
			cancelBtn = cfgButtonStyle.Render(" C  Cancel ")
		}

		dialog := cfgDialogStyle.BorderForeground(cfgRed).Render(
			cfgErrorStyle.Render("Unsaved Changes") + "\n\n" +
				saveBtn + "  " + discardBtn + "  " + cancelBtn + "\n\n" +
				cfgHintStyle.Render("← → select · Enter confirm"),
		)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
	}

	// Language picker dialog
	if m.showLanguagePicker {
		var dialogContent strings.Builder
		dialogContent.WriteString(cfgSectionStyle.Render("Select Language") + "\n\n")

		// Auto option
		autoLabel := "Auto (detect from system)"
		if m.languageCursor == 0 {
			dialogContent.WriteString(cfgSelectedStyle.Render(" ▸ "+autoLabel+" ") + "\n")
		} else {
			dialogContent.WriteString("   " + cfgValueStyle.Render(autoLabel) + "\n")
		}

		// Language options
		for i, lang := range i18n.SupportedLanguages {
			label := fmt.Sprintf("%s - %s", i18n.DisplayName(lang), lang)
			if m.languageCursor == i+1 {
				dialogContent.WriteString(cfgSelectedStyle.Render(" ▸ "+label+" ") + "\n")
			} else {
				dialogContent.WriteString("   " + cfgValueStyle.Render(label) + "\n")
			}
		}

		dialogContent.WriteString("\n" + cfgHintStyle.Render("↑↓ select · Enter confirm · Esc cancel"))

		dialog := cfgDialogStyle.Render(dialogContent.String())
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
	}

	return b.String()
}

func RunConfigTUI() error {
	p := tea.NewProgram(NewConfigTUI(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
