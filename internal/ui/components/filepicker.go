package components

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FileSelectedMsg is sent when a file is selected
type FileSelectedMsg struct {
	Path string
	Name string
	Size int64
}

// FilePickerCancelledMsg is sent when the picker is cancelled
type FilePickerCancelledMsg struct{}

// FileEntry represents a file or directory in the picker
type FileEntry struct {
	Name  string
	Path  string
	Size  int64
	IsDir bool
}

// FilePicker is a file browser component
type FilePicker struct {
	currentDir string
	entries    []FileEntry
	cursor     int
	width      int
	height     int
	err        error
	showHidden bool
}

// NewFilePicker creates a new file picker starting at the home directory
func NewFilePicker() FilePicker {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/"
	}

	fp := FilePicker{
		currentDir: home,
		width:      80,
		height:     24,
		showHidden: false,
	}
	fp.loadDirectory()
	return fp
}

// NewFilePickerAt creates a new file picker starting at the specified directory
func NewFilePickerAt(dir string) FilePicker {
	fp := FilePicker{
		currentDir: dir,
		width:      80,
		height:     24,
		showHidden: false,
	}
	fp.loadDirectory()
	return fp
}

func (fp *FilePicker) loadDirectory() {
	fp.entries = make([]FileEntry, 0)
	fp.err = nil

	// Add parent directory entry if not at root
	if fp.currentDir != "/" {
		fp.entries = append(fp.entries, FileEntry{
			Name:  "..",
			Path:  filepath.Dir(fp.currentDir),
			IsDir: true,
		})
	}

	dirEntries, err := os.ReadDir(fp.currentDir)
	if err != nil {
		fp.err = err
		return
	}

	// Separate directories and files
	var dirs, files []FileEntry

	for _, entry := range dirEntries {
		name := entry.Name()

		// Skip hidden files unless showHidden is true
		if !fp.showHidden && strings.HasPrefix(name, ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fe := FileEntry{
			Name:  name,
			Path:  filepath.Join(fp.currentDir, name),
			Size:  info.Size(),
			IsDir: entry.IsDir(),
		}

		if entry.IsDir() {
			dirs = append(dirs, fe)
		} else {
			files = append(files, fe)
		}
	}

	// Sort directories and files alphabetically (case-insensitive)
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	// Directories first, then files
	fp.entries = append(fp.entries, dirs...)
	fp.entries = append(fp.entries, files...)

	// Reset cursor if out of bounds
	if fp.cursor >= len(fp.entries) {
		fp.cursor = 0
	}
}

// SetSize sets the picker dimensions
func (fp *FilePicker) SetSize(width, height int) {
	fp.width = width
	fp.height = height
}

// Init returns the initial command
func (fp FilePicker) Init() tea.Cmd {
	return nil
}

// Update handles key events
func (fp FilePicker) Update(msg tea.Msg) (FilePicker, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if fp.cursor > 0 {
				fp.cursor--
			}
		case "down", "j":
			if fp.cursor < len(fp.entries)-1 {
				fp.cursor++
			}
		case "enter":
			if fp.cursor >= 0 && fp.cursor < len(fp.entries) {
				entry := fp.entries[fp.cursor]
				if entry.IsDir {
					// Navigate into directory
					fp.currentDir = entry.Path
					fp.cursor = 0
					fp.loadDirectory()
				} else {
					// Select file
					return fp, func() tea.Msg {
						return FileSelectedMsg{
							Path: entry.Path,
							Name: entry.Name,
							Size: entry.Size,
						}
					}
				}
			}
		case "backspace":
			// Go to parent directory
			if fp.currentDir != "/" {
				fp.currentDir = filepath.Dir(fp.currentDir)
				fp.cursor = 0
				fp.loadDirectory()
			}
		case "~":
			// Go to home directory
			home, err := os.UserHomeDir()
			if err == nil {
				fp.currentDir = home
				fp.cursor = 0
				fp.loadDirectory()
			}
		case ".":
			// Toggle hidden files
			fp.showHidden = !fp.showHidden
			fp.loadDirectory()
		case "esc", "q":
			return fp, func() tea.Msg {
				return FilePickerCancelledMsg{}
			}
		case "pgup":
			// Page up
			fp.cursor -= 10
			if fp.cursor < 0 {
				fp.cursor = 0
			}
		case "pgdown":
			// Page down
			fp.cursor += 10
			if fp.cursor >= len(fp.entries) {
				fp.cursor = len(fp.entries) - 1
			}
		case "home", "g":
			fp.cursor = 0
		case "end", "G":
			fp.cursor = len(fp.entries) - 1
		}
	}
	return fp, nil
}

// View renders the file picker
func (fp FilePicker) View() string {
	var b strings.Builder

	// Calculate visible area
	listHeight := fp.height - 12
	if listHeight < 5 {
		listHeight = 5
	}

	// Calculate scroll window
	start := 0
	if fp.cursor >= listHeight {
		start = fp.cursor - listHeight + 1
	}
	end := start + listHeight
	if end > len(fp.entries) {
		end = len(fp.entries)
	}

	// Error message if directory couldn't be read
	if fp.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(Danger)
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", fp.err)))
		b.WriteString("\n")
	}

	// Render entries
	for i := start; i < end; i++ {
		entry := fp.entries[i]

		var icon string
		if entry.IsDir {
			icon = "📁 "
		} else {
			icon = "📄 "
		}

		// Format: icon + name + size (right-aligned for files)
		var line string
		if entry.IsDir {
			line = icon + entry.Name + "/"
		} else {
			line = fmt.Sprintf("%s%-30s %s", icon, truncateName(entry.Name, 30), formatFileSize(entry.Size))
		}

		style := lipgloss.NewStyle().Padding(0, 1)

		if i == fp.cursor {
			style = style.
				Bold(true).
				Foreground(Text).
				Background(Primary)
		} else if entry.IsDir {
			style = style.Foreground(Primary)
		} else {
			style = style.Foreground(Text)
		}

		b.WriteString(style.Render(line))

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Empty directory message
	if len(fp.entries) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(Muted).Italic(true)
		b.WriteString(emptyStyle.Render("  (empty directory)"))
	}

	// Build the full view
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	pathStyle := lipgloss.NewStyle().
		Foreground(Muted).
		MarginBottom(1)

	hintStyle := lipgloss.NewStyle().
		Foreground(Muted).
		MarginTop(1)

	// Truncate path if too long
	displayPath := fp.currentDir
	maxPathLen := fp.width - 20
	if maxPathLen < 20 {
		maxPathLen = 20
	}
	if len(displayPath) > maxPathLen {
		displayPath = "..." + displayPath[len(displayPath)-maxPathLen+3:]
	}

	hiddenStatus := ""
	if fp.showHidden {
		hiddenStatus = " (showing hidden)"
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Select File"),
		pathStyle.Render(displayPath+hiddenStatus),
		"",
		b.String(),
		"",
		hintStyle.Render("↑/↓ navigate • enter select • backspace parent • ~ home • . toggle hidden • esc cancel"),
	)

	// Calculate container width
	containerWidth := fp.width - 8
	if containerWidth < 40 {
		containerWidth = 40
	}

	// Center in the available space
	return lipgloss.Place(
		fp.width,
		fp.height-4,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(1, 2).
			Width(containerWidth).
			Render(content),
	)
}

// CurrentDir returns the current directory path
func (fp FilePicker) CurrentDir() string {
	return fp.currentDir
}

// truncateName truncates a filename to maxLen characters
func truncateName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	// Keep extension visible
	ext := filepath.Ext(name)
	if len(ext) > 0 && len(ext) < maxLen-3 {
		base := name[:len(name)-len(ext)]
		availLen := maxLen - len(ext) - 3
		if availLen > 0 {
			return base[:availLen] + "..." + ext
		}
	}
	return name[:maxLen-3] + "..."
}
