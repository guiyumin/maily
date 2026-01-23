package components

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"maily/internal/i18n"
	"maily/internal/mail"
)

// Label i18n keys for system folders (Gmail and other providers)
var labelI18nKeys = map[string]string{
	// Standard
	mail.INBOX: "label.inbox",
	// Gmail
	mail.GmailStarred: "label.starred",
	mail.GmailSent:    "label.sent",
	mail.GmailDrafts:  "label.drafts",
	mail.GmailSpam:    "label.spam",
	mail.GmailTrash:   "label.trash",
	mail.GmailAllMail: "label.all_mail",
	// Yahoo / Standard IMAP
	mail.Sent:     "label.sent",
	mail.Draft:    "label.drafts",
	mail.Drafts:   "label.drafts",
	mail.Trash:    "label.trash",
	mail.Spam:     "label.spam",
	mail.BulkMail: "label.spam",
	mail.Archive:  "label.archive",
	mail.Junk:     "label.spam",
}

// System folder sort order (lower = higher priority)
var folderSortOrder = map[string]int{
	// Standard
	mail.INBOX: 0,
	// Gmail
	mail.GmailStarred: 1,
	mail.GmailSent:    2,
	mail.GmailDrafts:  3,
	mail.GmailAllMail: 4,
	mail.GmailSpam:    5,
	mail.GmailTrash:   6,
	// Yahoo / Standard IMAP
	mail.Sent:     2,
	mail.Draft:    3,
	mail.Drafts:   3,
	mail.Archive:  4,
	mail.Spam:     5,
	mail.BulkMail: 5,
	mail.Junk:     5,
	mail.Trash:    6,
}

// LabelPicker is a full-screen view for selecting a label/folder
type LabelPicker struct {
	folders  []string // System folders (INBOX, [Gmail]/*)
	labels   []string // Custom labels
	items    []pickerItem
	cursor   int
	selected string // Currently selected label (raw name)
	width    int
	height   int
}

type pickerItem struct {
	label     string // Raw IMAP label name (empty for headers)
	display   string // Display text
	isHeader  bool   // True if this is a section header
	isFolder  bool   // True if this is a system folder
}

func NewLabelPicker() LabelPicker {
	return LabelPicker{
		folders:  []string{mail.INBOX},
		labels:   []string{},
		selected: mail.INBOX,
		width:    80,
		height:   24,
	}
}

// Standard IMAP folder names that should be treated as system folders
var systemFolders = map[string]bool{
	mail.INBOX:    true,
	mail.Sent:     true,
	mail.Draft:    true,
	mail.Drafts:   true,
	mail.Trash:    true,
	mail.Spam:     true,
	mail.BulkMail: true,
	mail.Archive:  true,
	mail.Junk:     true,
}

func (p *LabelPicker) SetLabels(labels []string) {
	folders := make([]string, 0)
	customLabels := make([]string, 0)

	for _, label := range labels {
		if label == "[Gmail]" {
			// Skip the parent container - not a real folder
			continue
		}
		// Check if it's a system folder (Gmail or standard IMAP)
		if strings.HasPrefix(label, mail.GmailFolderPrefix) || systemFolders[label] {
			folders = append(folders, label)
		} else {
			customLabels = append(customLabels, label)
		}
	}

	// Sort folders by priority
	sort.Slice(folders, func(i, j int) bool {
		orderI, okI := folderSortOrder[folders[i]]
		orderJ, okJ := folderSortOrder[folders[j]]
		if !okI {
			orderI = 100 // Unknown [Gmail]/* folders go last
		}
		if !okJ {
			orderJ = 100
		}
		return orderI < orderJ
	})

	// Sort custom labels alphabetically
	sort.Strings(customLabels)

	p.folders = folders
	p.labels = customLabels

	// Build items list with headers
	p.buildItems()

	// Position cursor on currently selected label
	p.moveCursorToSelected()
}

func (p *LabelPicker) buildItems() {
	p.items = make([]pickerItem, 0)

	// Folders section
	if len(p.folders) > 0 {
		p.items = append(p.items, pickerItem{
			display:  i18n.T("label.folders"),
			isHeader: true,
		})
		for _, f := range p.folders {
			p.items = append(p.items, pickerItem{
				label:    f,
				display:  getDisplayName(f),
				isFolder: true,
			})
		}
	}

	// Labels section
	if len(p.labels) > 0 {
		p.items = append(p.items, pickerItem{
			display:  i18n.T("label.labels"),
			isHeader: true,
		})
		for _, l := range p.labels {
			p.items = append(p.items, pickerItem{
				label:   l,
				display: l,
			})
		}
	}

	// Reset cursor if out of bounds
	if p.cursor >= len(p.items) {
		p.cursor = 0
	}
	// Skip header if cursor is on one
	p.skipHeaders(1)
}

func (p *LabelPicker) moveCursorToSelected() {
	for i, item := range p.items {
		if item.label == p.selected {
			p.cursor = i
			return
		}
	}
	// Default to first non-header item
	p.cursor = 0
	p.skipHeaders(1)
}

func (p *LabelPicker) skipHeaders(direction int) {
	for p.cursor >= 0 && p.cursor < len(p.items) && p.items[p.cursor].isHeader {
		p.cursor += direction
	}
	// Bounds check
	if p.cursor < 0 {
		p.cursor = 0
		for p.cursor < len(p.items) && p.items[p.cursor].isHeader {
			p.cursor++
		}
	}
	if p.cursor >= len(p.items) {
		p.cursor = len(p.items) - 1
		for p.cursor >= 0 && p.items[p.cursor].isHeader {
			p.cursor--
		}
	}
}

func (p *LabelPicker) SetSelected(label string) {
	p.selected = label
	p.moveCursorToSelected()
}

func (p *LabelPicker) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p LabelPicker) Update(msg tea.Msg) (LabelPicker, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
				p.skipHeaders(-1)
			}
		case "down", "j":
			if p.cursor < len(p.items)-1 {
				p.cursor++
				p.skipHeaders(1)
			}
		}
	}
	return p, nil
}

func (p LabelPicker) View() string {
	var b strings.Builder

	// Calculate visible area
	listHeight := p.height - 10
	if listHeight < 5 {
		listHeight = 5
	}

	// Calculate scroll window
	start := 0
	if p.cursor >= listHeight {
		start = p.cursor - listHeight + 1
	}
	end := start + listHeight
	if end > len(p.items) {
		end = len(p.items)
	}

	// Render items
	for i := start; i < end; i++ {
		item := p.items[i]

		if item.isHeader {
			// Section header
			headerStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(Muted).
				Padding(0, 2).
				MarginTop(1)
			if i == start {
				headerStyle = headerStyle.MarginTop(0)
			}
			b.WriteString(headerStyle.Render(item.display))
		} else {
			// Selectable item
			prefix := "  "
			if item.label == p.selected {
				prefix = "● "
			}

			style := lipgloss.NewStyle().Padding(0, 2)

			if i == p.cursor {
				style = style.
					Bold(true).
					Foreground(Text).
					Background(Primary)
			} else if item.label == p.selected {
				style = style.
					Bold(true).
					Foreground(Primary)
			} else {
				style = style.Foreground(Text)
			}

			b.WriteString(style.Render(prefix + item.display))
		}

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Build the full view
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	hintStyle := lipgloss.NewStyle().
		Foreground(Muted).
		MarginTop(1)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(i18n.T("label.select")),
		"",
		b.String(),
		"",
		hintStyle.Render("↑/↓ "+i18n.T("help.navigate")+" • enter "+i18n.T("help.select")+" • esc "+i18n.T("help.cancel")),
	)

	// Center in the available space
	return lipgloss.Place(
		p.width,
		p.height-4,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(1, 3).
			Render(content),
	)
}

// CursorLabel returns the label at cursor position
func (p LabelPicker) CursorLabel() string {
	if p.cursor >= 0 && p.cursor < len(p.items) && !p.items[p.cursor].isHeader {
		return p.items[p.cursor].label
	}
	return mail.INBOX
}

// SelectedLabel returns the currently selected label
func (p LabelPicker) SelectedLabel() string {
	return p.selected
}

// getDisplayName returns a friendly display name for a label
func getDisplayName(label string) string {
	if key, ok := labelI18nKeys[label]; ok {
		return i18n.T(key)
	}
	// For [Gmail]/Something not in our map, strip the prefix
	if after, found := strings.CutPrefix(label, mail.GmailFolderPrefix); found {
		return after
	}
	return label
}

// GetLabelDisplayName is exported for use in views
func GetLabelDisplayName(label string) string {
	return getDisplayName(label)
}
