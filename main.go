package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"maily/internal/auth"
	"maily/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		runTUI()
		return
	}

	switch os.Args[1] {
	case "login":
		if len(os.Args) < 3 {
			fmt.Println("Usage: maily login <provider>")
			fmt.Println()
			fmt.Println("Providers:")
			fmt.Println("  gmail    Login with Gmail")
			os.Exit(1)
		}
		handleLogin(os.Args[2])

	case "logout":
		handleLogout()

	case "accounts":
		handleAccounts()

	case "search":
		handleSearch()

	case "help", "--help", "-h":
		printHelp()

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

func runTUI() {
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

	p := tea.NewProgram(
		ui.NewApp(store),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

func handleLogin(provider string) {
	switch provider {
	case "gmail":
		loginGmail()
	default:
		fmt.Printf("Unknown provider: %s\n", provider)
		fmt.Println()
		fmt.Println("Available providers:")
		fmt.Println("  gmail    Login with Gmail")
		os.Exit(1)
	}
}

func loginGmail() {
	p := tea.NewProgram(
		ui.NewLoginApp("gmail"),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func handleLogout() {
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if len(store.Accounts) == 0 {
		fmt.Println("No accounts configured.")
		return
	}

	// If specific account provided
	if len(os.Args) >= 3 {
		email := os.Args[2]
		if store.RemoveAccount(email) {
			store.Save()
			fmt.Printf("✓ Removed account %s\n", email)
		} else {
			fmt.Printf("Account not found: %s\n", email)
		}
		return
	}

	// If only one account, remove it
	if len(store.Accounts) == 1 {
		email := store.Accounts[0].Credentials.Email
		store.RemoveAccount(email)
		store.Save()
		fmt.Printf("✓ Removed account %s\n", email)
		return
	}

	// Multiple accounts - show list
	fmt.Println()
	fmt.Println("  Which account to remove?")
	fmt.Println()
	for i, acc := range store.Accounts {
		fmt.Printf("  %d. %s\n", i+1, acc.Credentials.Email)
	}
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("  Enter number (or 0 to cancel): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	num, err := strconv.Atoi(input)
	if err != nil || num < 0 || num > len(store.Accounts) {
		fmt.Println("Cancelled.")
		return
	}

	if num == 0 {
		fmt.Println("Cancelled.")
		return
	}

	email := store.Accounts[num-1].Credentials.Email
	store.RemoveAccount(email)
	store.Save()
	fmt.Printf("✓ Removed account %s\n", email)
}

func handleAccounts() {
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if len(store.Accounts) == 0 {
		fmt.Println("No accounts configured.")
		fmt.Println()
		fmt.Println("Run: maily login gmail")
		return
	}

	fmt.Println()
	fmt.Println("  Accounts:")
	fmt.Println("  ─────────")
	for _, acc := range store.Accounts {
		fmt.Printf("  • %s (%s)\n", acc.Credentials.Email, acc.Provider)
	}
	fmt.Println()
}

func handleSearch() {
	searchCmd := flag.NewFlagSet("search", flag.ExitOnError)
	fromFlag := searchCmd.String("from", "", "Account email to search from")
	queryFlag := searchCmd.String("query", "", "Gmail search query (uses Gmail syntax)")

	searchCmd.Usage = func() {
		fmt.Println("Usage: maily search --from=<account> --query=\"<query>\"")
		fmt.Println()
		fmt.Println("Options:")
		searchCmd.PrintDefaults()
		fmt.Println()
		fmt.Println("Gmail search syntax examples:")
		fmt.Println("  from:sender@example.com    Emails from a sender")
		fmt.Println("  subject:hello              Emails with subject containing 'hello'")
		fmt.Println("  has:attachment             Emails with attachments")
		fmt.Println("  is:unread                  Unread emails")
		fmt.Println("  older_than:30d             Emails older than 30 days")
		fmt.Println("  category:promotions        Promotional emails")
		fmt.Println("  larger:5M                  Emails larger than 5MB")
		fmt.Println()
		fmt.Println("Example:")
		fmt.Println("  maily search --from=me@gmail.com --query=\"category:promotions older_than:30d\"")
	}

	if err := searchCmd.Parse(os.Args[2:]); err != nil {
		os.Exit(1)
	}

	if *queryFlag == "" {
		fmt.Println("Error: --query is required")
		fmt.Println()
		searchCmd.Usage()
		os.Exit(1)
	}

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

	// Find the account
	var account *auth.Account
	if *fromFlag == "" {
		if len(store.Accounts) == 1 {
			account = &store.Accounts[0]
		} else {
			fmt.Println("Error: --from is required when multiple accounts are configured")
			fmt.Println()
			fmt.Println("Available accounts:")
			for _, acc := range store.Accounts {
				fmt.Printf("  - %s\n", acc.Credentials.Email)
			}
			os.Exit(1)
		}
	} else {
		account = store.GetAccount(*fromFlag)
		if account == nil {
			fmt.Printf("Error: account '%s' not found\n", *fromFlag)
			fmt.Println()
			fmt.Println("Available accounts:")
			for _, acc := range store.Accounts {
				fmt.Printf("  - %s\n", acc.Credentials.Email)
			}
			os.Exit(1)
		}
	}

	// Run the search TUI
	p := tea.NewProgram(
		ui.NewSearchApp(account, *queryFlag),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running search: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("maily - A terminal email client")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  maily                Start the email client")
	fmt.Println("  maily login gmail    Add a Gmail account")
	fmt.Println("  maily accounts       List all accounts")
	fmt.Println("  maily logout         Remove an account")
	fmt.Println("  maily search         Search emails with Gmail query syntax")
	fmt.Println("  maily help           Show this help")
	fmt.Println()
	fmt.Println("Keyboard shortcuts (in client):")
	fmt.Println("  tab      Switch account")
	fmt.Println("  j/k      Navigate up/down")
	fmt.Println("  enter    Open email")
	fmt.Println("  esc      Go back")
	fmt.Println("  r        Refresh")
	fmt.Println("  l        Load more")
	fmt.Println("  d        Delete")
	fmt.Println("  q        Quit")
}
