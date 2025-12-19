package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"maily/internal/ui"
)

var loginCmd = &cobra.Command{
	Use:   "login [provider]",
	Short: "Add an email account",
	Long:  "Add an email account. Currently supports: gmail",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		handleLogin(args[0])
	},
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
