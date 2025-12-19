package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"cocomail/internal/auth"
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

	case "help", "--help", "-h":
		printHelp()

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

func runTUI() {
	creds, err := auth.LoadCredentials()
	if err != nil {
		fmt.Printf("Error loading credentials: %v\n", err)
		os.Exit(1)
	}

	if creds == nil {
		fmt.Println("Not logged in. Run:")
		fmt.Println()
		fmt.Println("  cocomail login gmail")
		fmt.Println()
		os.Exit(1)
	}

	p := tea.NewProgram(
		ui.NewApp(creds),
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
	existing, _ := auth.LoadCredentials()
	if existing != nil {
		fmt.Printf("Already logged in as %s\n", existing.Email)
		fmt.Println("Run 'cocomail logout' first to switch accounts.")
		return
	}

	creds, err := auth.PromptGmailCredentials()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := auth.SaveCredentials(creds); err != nil {
		fmt.Printf("Error saving credentials: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("✓ Logged in as %s\n", creds.Email)
	fmt.Println()
	fmt.Println("Run 'cocomail' to start.")
}

func handleLogout() {
	creds, _ := auth.LoadCredentials()
	if creds == nil {
		fmt.Println("Not logged in.")
		return
	}

	if err := auth.DeleteCredentials(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Logged out from %s\n", creds.Email)
}

func printHelp() {
	fmt.Println("cocomail - A terminal email client")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  cocomail              Start the email client")
	fmt.Println("  cocomail login gmail  Login with Gmail")
	fmt.Println("  cocomail logout       Logout and remove credentials")
	fmt.Println("  cocomail help         Show this help")
	fmt.Println()
	fmt.Println("Keyboard shortcuts (in client):")
	fmt.Println("  j/k      Navigate up/down")
	fmt.Println("  enter    Open email")
	fmt.Println("  esc      Go back")
	fmt.Println("  r        Refresh")
	fmt.Println("  d        Delete")
	fmt.Println("  q        Quit")
}
