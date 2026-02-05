package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"maily/internal/i18n"
)

// Command represents a slash command
type Command struct {
	Name        string
	DescKey     string   // i18n key for description
	Shortcut    string   // keyboard shortcut hint
	Views       []string // views where this command is available: "list", "read", "today"
}

// Description returns the translated description
func (c Command) Description() string {
	return i18n.T(c.DescKey)
}

// CommandSelectedMsg is sent when a command is selected
type CommandSelectedMsg struct {
	Command string
}

// AllCommands defines all available slash commands
var AllCommands = []Command{
	{Name: "new", DescKey: "command.new", Shortcut: "n", Views: []string{"list"}},
	{Name: "reply", DescKey: "command.reply", Shortcut: "r", Views: []string{"list", "today"}},
	{Name: "reply-all", DescKey: "command.reply_all", Shortcut: "A", Views: []string{"list", "today"}},
	{Name: "delete", DescKey: "command.delete", Shortcut: "d", Views: []string{"list", "today"}},
	{Name: "search", DescKey: "command.search", Shortcut: "s", Views: []string{"list"}},
	{Name: "refresh", DescKey: "command.refresh", Shortcut: "R", Views: []string{"list"}},
	{Name: "labels", DescKey: "command.labels", Shortcut: "f", Views: []string{"list"}},
	{Name: "summarize", DescKey: "command.summarize", Shortcut: "s", Views: []string{"today"}},
	{Name: "event", DescKey: "command.event", Shortcut: "e", Views: []string{"today"}},
	{Name: "add", DescKey: "command.add", Shortcut: "a", Views: []string{"today"}},
}

// CommandPalette is the command palette component
type CommandPalette struct {
	input      textinput.Model
	commands   []Command // filtered commands for current view
	allForView []Command // all commands for current view
	cursor     int
	width      int
	height     int
	currentView string
}

// NewCommandPalette creates a new command palette
func NewCommandPalette() CommandPalette {
	ti := textinput.New()
	ti.Placeholder = i18n.T("command.palette.placeholder")
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40

	return CommandPalette{
		input:    ti,
		commands: []Command{},
		cursor:   0,
	}
}

// SetView sets the current view and filters commands accordingly
func (c *CommandPalette) SetView(view string) {
	c.currentView = view
	c.allForView = filterCommandsByView(AllCommands, view)
	c.commands = c.allForView
	c.cursor = 0
	c.input.SetValue("")
}

// SetSize sets the palette dimensions
func (c *CommandPalette) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// filterCommandsByView returns commands available for the given view
func filterCommandsByView(commands []Command, view string) []Command {
	var filtered []Command
	for _, cmd := range commands {
		for _, v := range cmd.Views {
			if v == view {
				filtered = append(filtered, cmd)
				break
			}
		}
	}
	return filtered
}

// filterCommandsByQuery filters commands by search query (fuzzy)
func filterCommandsByQuery(commands []Command, query string) []Command {
	if query == "" {
		return commands
	}
	query = strings.ToLower(query)
	var filtered []Command
	for _, cmd := range commands {
		if strings.Contains(strings.ToLower(cmd.Name), query) ||
			strings.Contains(strings.ToLower(cmd.Description()), query) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

func (c CommandPalette) Init() tea.Cmd {
	return textinput.Blink
}

func (c CommandPalette) Update(msg tea.Msg) (CommandPalette, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "ctrl+p":
			if c.cursor > 0 {
				c.cursor--
			}
			return c, nil
		case "down", "ctrl+n":
			if c.cursor < len(c.commands)-1 {
				c.cursor++
			}
			return c, nil
		case "enter":
			if len(c.commands) > 0 && c.cursor < len(c.commands) {
				return c, func() tea.Msg {
					return CommandSelectedMsg{Command: c.commands[c.cursor].Name}
				}
			}
			return c, nil
		}
	}

	// Update text input
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)

	// Filter commands based on input
	c.commands = filterCommandsByQuery(c.allForView, c.input.Value())
	if c.cursor >= len(c.commands) {
		c.cursor = max(0, len(c.commands)-1)
	}

	return c, cmd
}

func (c CommandPalette) View() string {
	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		Render(i18n.T("command.palette.title"))

	// Input field with / prefix
	inputLine := lipgloss.NewStyle().
		Foreground(Primary).
		Render("/") + c.input.View()

	// Command list
	var cmdLines []string
	for i, cmd := range c.commands {
		name := cmd.Name
		desc := cmd.Description()
		shortcut := cmd.Shortcut

		nameStyle := lipgloss.NewStyle().Width(12)
		descStyle := lipgloss.NewStyle().Foreground(TextDim)
		shortcutStyle := lipgloss.NewStyle().Foreground(Primary).Width(4)

		line := nameStyle.Render(name) + " " + descStyle.Render(desc)
		if shortcut != "" {
			line = shortcutStyle.Render("["+shortcut+"]") + " " + line
		}

		if i == c.cursor {
			line = lipgloss.NewStyle().
				Background(Primary).
				Foreground(Text).
				Render("> " + line)
		} else {
			line = "  " + line
		}
		cmdLines = append(cmdLines, line)
	}

	// Join command lines
	cmdList := strings.Join(cmdLines, "\n")
	if len(c.commands) == 0 {
		cmdList = lipgloss.NewStyle().
			Foreground(TextDim).
			Italic(true).
			Render("  " + i18n.T("command.no_matching"))
	}

	// Container
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		inputLine,
		"",
		cmdList,
	)

	// Box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(1, 2).
		Width(60)

	return boxStyle.Render(content)
}

// SelectedCommand returns the currently highlighted command name
func (c CommandPalette) SelectedCommand() string {
	if len(c.commands) > 0 && c.cursor < len(c.commands) {
		return c.commands[c.cursor].Name
	}
	return ""
}
