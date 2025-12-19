package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"cocomail/internal/auth"
	"cocomail/internal/gmail"
	"cocomail/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		runTUI()
		return
	}

	switch os.Args[1] {
	case "login":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cocomail login <provider>")
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

	case "switch":
		handleSwitch()

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
		fmt.Println("  cocomail login gmail")
		fmt.Println()
		os.Exit(1)
	}

	// If multiple accounts and none active, let user choose
	var account *auth.Account
	if len(store.Accounts) > 1 && store.Active == "" {
		account = promptAccountSelection(store)
		store.SetActive(account.Credentials.Email)
		store.Save()
	} else {
		account = store.GetActiveAccount()
	}

	if account == nil {
		fmt.Println("No active account. Run:")
		fmt.Println()
		fmt.Println("  cocomail login gmail")
		fmt.Println()
		os.Exit(1)
	}

	p := tea.NewProgram(
		ui.NewApp(&account.Credentials, account.Credentials.Email),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

func promptAccountSelection(store *auth.AccountStore) *auth.Account {
	fmt.Println()
	fmt.Println("  Select an account:")
	fmt.Println("  ──────────────────")
	fmt.Println()

	for i, acc := range store.Accounts {
		fmt.Printf("  %d. %s (%s)\n", i+1, acc.Credentials.Email, acc.Provider)
	}
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("  Enter number: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		num, err := strconv.Atoi(input)
		if err != nil || num < 1 || num > len(store.Accounts) {
			fmt.Println("  Invalid selection. Try again.")
			continue
		}

		return &store.Accounts[num-1]
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
	account, err := auth.PromptGmailCredentials()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Test connection before saving
	fmt.Println()
	fmt.Print("  Verifying credentials...")

	client, err := gmail.NewIMAPClient(&account.Credentials)
	if err != nil {
		fmt.Println(" ✗")
		fmt.Println()
		fmt.Printf("  Login failed: %v\n", err)
		fmt.Println()
		fmt.Println("  Make sure you:")
		fmt.Println("  • Used an App Password (not your regular password)")
		fmt.Println("  • Have IMAP enabled in Gmail settings")
		fmt.Println()
		os.Exit(1)
	}
	client.Close()
	fmt.Println(" ✓")

	// Save to account store
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	store.AddAccount(*account)
	store.SetActive(account.Credentials.Email)

	if err := store.Save(); err != nil {
		fmt.Printf("Error saving account: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("  ✓ Logged in as %s\n", account.Credentials.Email)
	fmt.Println()
	fmt.Println("  Run 'cocomail' to start.")
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
		active := ""
		if acc.Credentials.Email == store.Active {
			active = " (active)"
		}
		fmt.Printf("  %d. %s%s\n", i+1, acc.Credentials.Email, active)
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
		fmt.Println("Run: cocomail login gmail")
		return
	}

	fmt.Println()
	fmt.Println("  Accounts:")
	fmt.Println("  ─────────")
	for _, acc := range store.Accounts {
		active := ""
		if acc.Credentials.Email == store.Active {
			active = " ← active"
		}
		fmt.Printf("  • %s (%s)%s\n", acc.Credentials.Email, acc.Provider, active)
	}
	fmt.Println()
}

func handleSwitch() {
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if len(store.Accounts) == 0 {
		fmt.Println("No accounts configured.")
		return
	}

	if len(store.Accounts) == 1 {
		fmt.Println("Only one account configured.")
		return
	}

	// If specific account provided
	if len(os.Args) >= 3 {
		email := os.Args[2]
		if store.SetActive(email) {
			store.Save()
			fmt.Printf("✓ Switched to %s\n", email)
		} else {
			fmt.Printf("Account not found: %s\n", email)
		}
		return
	}

	// Show selection
	fmt.Println()
	fmt.Println("  Switch to which account?")
	fmt.Println()
	for i, acc := range store.Accounts {
		active := ""
		if acc.Credentials.Email == store.Active {
			active = " (current)"
		}
		fmt.Printf("  %d. %s%s\n", i+1, acc.Credentials.Email, active)
	}
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("  Enter number: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(store.Accounts) {
		fmt.Println("Cancelled.")
		return
	}

	email := store.Accounts[num-1].Credentials.Email
	store.SetActive(email)
	store.Save()
	fmt.Printf("✓ Switched to %s\n", email)
}

func printHelp() {
	fmt.Println("cocomail - A terminal email client")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  cocomail                Start the email client")
	fmt.Println("  cocomail login gmail    Add a Gmail account")
	fmt.Println("  cocomail accounts       List all accounts")
	fmt.Println("  cocomail switch         Switch active account")
	fmt.Println("  cocomail logout         Remove an account")
	fmt.Println("  cocomail help           Show this help")
	fmt.Println()
	fmt.Println("Keyboard shortcuts (in client):")
	fmt.Println("  j/k      Navigate up/down")
	fmt.Println("  enter    Open email")
	fmt.Println("  esc      Go back")
	fmt.Println("  r        Refresh")
	fmt.Println("  l        Load more")
	fmt.Println("  d        Delete")
	fmt.Println("  q        Quit")
}
