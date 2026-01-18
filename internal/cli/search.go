package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"maily/internal/auth"
	"maily/internal/client"
	"maily/internal/ui"
	"maily/internal/ui/utils"
)

var (
	searchAccount string
	searchQuery   string
	searchLimit   int
	searchOffset  int
	searchCount   bool
	searchFormat  string
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search emails",
	Long: `Search emails in your mailbox.

By default, launches an interactive TUI. Use --format to get non-interactive output
suitable for scripting and piping to other commands.

For Gmail accounts, full Gmail search syntax is supported:
  from:sender@example.com    Emails from a sender
  to:recipient@example.com   Emails to a recipient
  subject:hello              Emails with subject containing 'hello'
  has:attachment             Emails with attachments
  is:unread                  Unread emails
  is:starred                 Starred emails
  in:inbox                   Emails in inbox
  in:trash                   Emails in trash
  older_than:30d             Emails older than 30 days
  newer_than:7d              Emails newer than 7 days
  category:promotions        Promotional emails
  category:social            Social emails
  larger:5M                  Emails larger than 5MB
  label:important            Emails with label

For other providers (Yahoo, etc.), basic text search is used:
  Simply enter keywords to search in email body and headers.`,
	Example: `  # Interactive TUI search
  maily search -a me@gmail.com -q "from:temu"

  # Non-interactive: get count only
  maily search -q "from:temu" --count

  # Non-interactive: JSON output with pagination
  maily search -q "from:temu" --format=json --limit=50
  maily search -q "from:temu" --format=json --limit=50 --offset=50

  # Non-interactive: table output
  maily search -q "is:unread" --format=table`,
	Run: func(cmd *cobra.Command, args []string) {
		handleSearch(cmd)
	},
}

func init() {
	searchCmd.Flags().StringVarP(&searchAccount, "account", "a", "", "Account email to search")
	searchCmd.Flags().StringVarP(&searchQuery, "query", "q", "", "Search query")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 100, "Max results to return (default: 100)")
	searchCmd.Flags().IntVar(&searchOffset, "offset", 0, "Skip first N results for pagination")
	searchCmd.Flags().BoolVar(&searchCount, "count", false, "Only return the count, don't fetch emails")
	searchCmd.Flags().StringVar(&searchFormat, "format", "", "Output format: json or table (non-interactive)")
	searchCmd.MarkFlagRequired("query")
}

func handleSearch(cmd *cobra.Command) {
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Printf("Error loading accounts: %v\n", err)
		os.Exit(1)
	}

	if len(store.Accounts) == 0 {
		fmt.Println("No accounts configured. Run:")
		fmt.Println()
		fmt.Println("  maily login")
		fmt.Println()
		os.Exit(1)
	}

	var account *auth.Account
	if searchAccount == "" {
		if len(store.Accounts) == 1 {
			account = &store.Accounts[0]
		} else {
			fmt.Println("Error: --account (-a) is required when multiple accounts are configured")
			fmt.Println()
			fmt.Println("Available accounts:")
			for _, acc := range store.Accounts {
				fmt.Printf("  - %s\n", acc.Credentials.Email)
			}
			os.Exit(1)
		}
	} else {
		account = store.GetAccount(searchAccount)
		if account == nil {
			fmt.Printf("Error: account '%s' not found\n", searchAccount)
			fmt.Println()
			fmt.Println("Available accounts:")
			for _, acc := range store.Accounts {
				fmt.Printf("  - %s\n", acc.Credentials.Email)
			}
			os.Exit(1)
		}
	}

	// Non-interactive mode: any of --count, --format, --limit, --offset specified
	isNonInteractive := searchCount ||
		searchFormat != "" ||
		cmd.Flags().Changed("limit") ||
		cmd.Flags().Changed("offset")

	if isNonInteractive {
		handleNonInteractiveSearch(account)
		return
	}

	// Interactive TUI mode
	p := tea.NewProgram(
		ui.NewSearchApp(account, searchQuery),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running search: %v\n", err)
		os.Exit(1)
	}
}

// SearchResult represents a single email in search results
type SearchResult struct {
	UID           uint32 `json:"uid"`
	From          string `json:"from"`
	To            string `json:"to"`
	Subject       string `json:"subject"`
	Date          string `json:"date"`
	Unread        bool   `json:"unread"`
	HasAttachment bool   `json:"has_attachment"`
	Snippet       string `json:"snippet,omitempty"`
}

// SearchResponse represents the paginated search response
type SearchResponse struct {
	Total   int            `json:"total"`
	Offset  int            `json:"offset"`
	Limit   int            `json:"limit"`
	Results []SearchResult `json:"results"`
}

func handleNonInteractiveSearch(account *auth.Account) {
	// Validate format if specified
	if searchFormat != "" && searchFormat != "json" && searchFormat != "table" {
		fmt.Fprintf(os.Stderr, "Error: invalid format '%s'. Use 'json' or 'table'\n", searchFormat)
		os.Exit(1)
	}

	serverClient, err := client.Connect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to server: %v\n", err)
		os.Exit(1)
	}
	defer serverClient.Close()

	cached, err := serverClient.Search(account.Credentials.Email, "INBOX", searchQuery)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching: %v\n", err)
		os.Exit(1)
	}

	total := len(cached)

	// --count mode: just output the count
	if searchCount {
		if searchFormat == "json" {
			output, _ := json.Marshal(map[string]int{"total": total})
			fmt.Println(string(output))
		} else {
			fmt.Println(total)
		}
		return
	}

	// Apply pagination to UIDs
	start := searchOffset
	if start > total {
		start = total
	}
	end := start + searchLimit
	if end > total {
		end = total
	}

	paginated := cached[start:end]

	// Build response
	response := SearchResponse{
		Total:   total,
		Offset:  searchOffset,
		Limit:   searchLimit,
		Results: []SearchResult{},
	}

	if len(paginated) > 0 {
		for _, email := range paginated {
			response.Results = append(response.Results, SearchResult{
				UID:           uint32(email.UID),
				From:          email.From,
				To:            email.To,
				Subject:       email.Subject,
				Date:          email.Date.Format("2006-01-02T15:04:05Z07:00"),
				Unread:        email.Unread,
				HasAttachment: len(email.Attachments) > 0,
				Snippet:       truncateSnippet(email.Snippet, 100),
			})
		}
	}

	// Output based on format
	switch searchFormat {
	case "json":
		output, _ := json.MarshalIndent(response, "", "  ")
		fmt.Println(string(output))
	case "table":
		outputTable(response)
	default:
		// Default to JSON when format not specified but non-interactive
		output, _ := json.MarshalIndent(response, "", "  ")
		fmt.Println(string(output))
	}
}

func outputTable(response SearchResponse) {
	// Padding
	fmt.Println()
	pad := "  "

	// Header style
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#D1D5DB"))

	fmt.Println(pad + headerStyle.Render(fmt.Sprintf("Total: %d (showing %d-%d)", response.Total, response.Offset+1, response.Offset+len(response.Results))))
	fmt.Println()

	if len(response.Results) == 0 {
		fmt.Println(pad + "No results.")
		return
	}

	// Styles - brighter colors
	unreadDot := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Render("●")
	readDot := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render("○")
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	unreadStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	readStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
	attachStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))

	sep := separatorStyle.Render("│")

	for _, r := range response.Results {
		// Status indicator
		status := readDot
		if r.Unread {
			status = unreadDot
		}

		// Attachment indicator (📎 is 2 chars wide, space is 1, so use 2 spaces for alignment)
		attach := "  "
		if r.HasAttachment {
			attach = attachStyle.Render("📎")
		}

		// Format fields with fixed widths
		fromStr := utils.TruncateStr(utils.ExtractNameFromEmail(r.From), 20)
		subjectStr := utils.TruncateStr(r.Subject, 50)
		date := r.Date[:10]

		// Pad to fixed width using visual width (accounts for emojis/unicode)
		fromWidth := lipgloss.Width(fromStr)
		subjectWidth := lipgloss.Width(subjectStr)
		fromPadded := fromStr + strings.Repeat(" ", max(0, 20-fromWidth))
		subjectPadded := subjectStr + strings.Repeat(" ", max(0, 50-subjectWidth))

		// Apply style based on read status
		var fromStyled, subjectStyled, dateStyled string
		if r.Unread {
			fromStyled = unreadStyle.Render(fromPadded)
			subjectStyled = unreadStyle.Render(subjectPadded)
			dateStyled = unreadStyle.Render(date)
		} else {
			fromStyled = readStyle.Render(fromPadded)
			subjectStyled = readStyle.Render(subjectPadded)
			dateStyled = readStyle.Render(date)
		}

		fmt.Printf("%s%s %s %s %s %s %s %s\n", pad, status, attach, fromStyled, sep, subjectStyled, sep, dateStyled)
	}
	fmt.Println()
}

func truncateSnippet(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
