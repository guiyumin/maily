package utils

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// TruncateStr truncates a string to maxLen visual width using unicode ellipsis
func TruncateStr(s string, maxLen int) string {
	width := lipgloss.Width(s)
	if width <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	// Truncate rune by rune until we fit
	runes := []rune(s)
	for i := len(runes) - 1; i >= 0; i-- {
		truncated := string(runes[:i]) + "…"
		if lipgloss.Width(truncated) <= maxLen {
			return truncated
		}
	}
	return "…"
}

func ExtractNameFromEmail(from string) string {
	if idx := strings.Index(from, "<"); idx > 0 {
		return strings.TrimSpace(from[:idx])
	}
	return from
}

func FormatEmailDate(t time.Time) string {
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	if t.Year() == now.Year() {
		return t.Format("Jan 02")
	}
	return t.Format("02/01/06")
}

// PadRight pads a string to targetWidth using visual width (handles double-width chars)
func PadRight(s string, targetWidth int) string {
	currentWidth := lipgloss.Width(s)
	if currentWidth >= targetWidth {
		return s
	}
	return s + strings.Repeat(" ", targetWidth-currentWidth)
}
