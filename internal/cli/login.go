package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"maily/config"
	"maily/internal/i18n"
	"maily/internal/ui"
)

var loginCmd = &cobra.Command{
	Use:   "login [provider]",
	Short: "Add an email account",
	Long:  "Add an email account. Currently supports: gmail, yahoo, qq",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize i18n for login UI
		cfg, _ := config.Load()
		i18n.Init(cfg.Language)

		if len(args) == 0 {
			selectAndLogin()
		} else {
			handleLogin(args[0])
		}
	},
}

func selectAndLogin() {
	selector := ui.NewProviderSelector()
	p := tea.NewProgram(
		selector,
		tea.WithAltScreen(),
	)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if sel, ok := finalModel.(ui.ProviderSelector); ok && sel.Selected() != "" {
		loginWithProvider(sel.Selected())
	}
}

func handleLogin(provider string) {
	switch provider {
	case "gmail":
		loginWithProvider("gmail")
	case "yahoo":
		loginWithProvider("yahoo")
	case "qq":
		loginWithProvider("qq")
	default:
		fmt.Printf("Unknown provider: %s\n", provider)
		fmt.Println()
		fmt.Println("Available providers:")
		fmt.Println("  gmail    Login with Gmail")
		fmt.Println("  yahoo    Login with Yahoo Mail")
		fmt.Println("  qq       Login with QQ Mail")
		os.Exit(1)
	}
}

func loginWithProvider(provider string) {
	loginApp := ui.NewLoginApp(provider)
	p := tea.NewProgram(
		loginApp,
		tea.WithAltScreen(),
	)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// If login succeeded, go directly to email list
	if login, ok := finalModel.(ui.LoginApp); ok && login.Success() {
		runTUI()
	}
}
