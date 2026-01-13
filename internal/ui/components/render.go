package components

import (
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/charmbracelet/glamour"
)

// RenderHTMLBody converts HTML email body to terminal-friendly output
// using glamour for rich markdown rendering
func RenderHTMLBody(htmlBody string, width int) string {
	if htmlBody == "" {
		return ""
	}

	// Convert HTML to Markdown
	conv := converter.NewConverter()
	markdown, err := conv.ConvertString(htmlBody)
	if err != nil {
		// Fallback: strip tags and return plain text
		return stripHTMLTags(htmlBody)
	}

	// Render Markdown with glamour
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return markdown
	}

	rendered, err := renderer.Render(markdown)
	if err != nil {
		return markdown
	}

	return strings.TrimSpace(rendered)
}

// stripHTMLTags is a simple fallback HTML stripper
func stripHTMLTags(html string) string {
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
			result.WriteRune(' ')
		} else if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}
