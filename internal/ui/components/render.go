package components

import (
	"regexp"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var (
	styleRegex   = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	scriptRegex  = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	headRegex    = regexp.MustCompile(`(?is)<head[^>]*>.*?</head>`)
	imgRegex     = regexp.MustCompile(`(?is)<img[^>]*>`)
	multiNewline = regexp.MustCompile(`\n{3,}`)
	// Clean up markdown artifacts: image links, tracking pixels, link references
	imgLinkRegex = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	linkRefRegex = regexp.MustCompile(`(?m)^\[\d+\]:\s*https?://[^\s]*\.(png|jpg|jpeg|gif|webp|svg)[^\s]*$`)
	emptyLinkRef = regexp.MustCompile(`(?m)^\[\d+\]:\s*https?://[^\s]*(imgping|tracking|pixel)[^\s]*$`)
)

// RenderHTMLBody converts HTML email body to terminal-friendly output
func RenderHTMLBody(htmlBody string, width int) string {
	if htmlBody == "" {
		return ""
	}

	// Strip non-content HTML tags
	cleaned := styleRegex.ReplaceAllString(htmlBody, "")
	cleaned = scriptRegex.ReplaceAllString(cleaned, "")
	cleaned = headRegex.ReplaceAllString(cleaned, "")
	cleaned = imgRegex.ReplaceAllString(cleaned, "") // Images don't display in terminal

	// Convert HTML to Markdown (fresh converter each time to avoid state issues)
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
		),
	)
	markdown, err := conv.ConvertString(cleaned)
	if err != nil {
		return stripHTMLTags(cleaned)
	}

	// Clean up markdown artifacts
	markdown = imgLinkRegex.ReplaceAllString(markdown, "")   // Remove image links
	markdown = linkRefRegex.ReplaceAllString(markdown, "")   // Remove image URL references
	markdown = emptyLinkRef.ReplaceAllString(markdown, "")   // Remove tracking pixel references
	markdown = multiNewline.ReplaceAllString(markdown, "\n\n")

	// Render with glamour
	renderer, err := glamour.NewTermRenderer(
		glamour.WithColorProfile(lipgloss.ColorProfile()),
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
