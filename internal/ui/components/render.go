package components

import (
	"regexp"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/charmbracelet/glamour"
)

var (
	// Patterns to strip style/script tags and their contents
	styleRegex  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	scriptRegex = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	headRegex   = regexp.MustCompile(`(?is)<head[^>]*>.*?</head>`)
)

// RenderHTMLBody converts HTML email body to terminal-friendly output
// using glamour for rich markdown rendering
func RenderHTMLBody(htmlBody string, width int) string {
	if htmlBody == "" {
		return ""
	}

	// Strip style, script, and head tags before conversion
	cleaned := styleRegex.ReplaceAllString(htmlBody, "")
	cleaned = scriptRegex.ReplaceAllString(cleaned, "")
	cleaned = headRegex.ReplaceAllString(cleaned, "")

	// Convert HTML to Markdown
	conv := converter.NewConverter()
	markdown, err := conv.ConvertString(cleaned)
	if err != nil {
		// Fallback: strip tags and return plain text
		return stripHTMLTags(cleaned)
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
