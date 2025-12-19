package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"maily/internal/auth"
	"maily/internal/ui"
)

var (
	searchAccount string
	searchQuery   string
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search emails with Gmail query syntax",
	Long: `Search emails using Gmail's search syntax.

Gmail search syntax examples:
  from:sender@example.com    Emails from a sender
  subject:hello              Emails with subject containing 'hello'
  has:attachment             Emails with attachments
  is:unread                  Unread emails
  older_than:30d             Emails older than 30 days
  category:promotions        Promotional emails
  larger:5M                  Emails larger than 5MB`,
	Example: `  maily search -a me@gmail.com -q "category:promotions older_than:30d"
  maily search --account=me@gmail.com --query="is:unread"`,
	Run: func(cmd *cobra.Command, args []string) {
		handleSearch()
	},
}

func init() {
	searchCmd.Flags().StringVarP(&searchAccount, "account", "a", "", "Account email to search")
	searchCmd.Flags().StringVarP(&searchQuery, "query", "q", "", "Gmail search query (uses Gmail syntax)")
	searchCmd.MarkFlagRequired("query")
}

func handleSearch() {
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Printf("Error loading accounts: %v\n", err)
		os.Exit(1)
	}

	if len(store.Accounts) == 0 {
		fmt.Println("No accounts configured. Run:")
		fmt.Println()
		fmt.Println("  maily login gmail")
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
