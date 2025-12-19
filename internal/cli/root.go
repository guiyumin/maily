package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"maily/internal/auth"
	"maily/internal/ui"
)

var rootCmd = &cobra.Command{
	Use:   "maily",
	Short: "A terminal email client",
	Long:  "maily - A terminal email client for Gmail",
	Run: func(cmd *cobra.Command, args []string) {
		runTUI()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(accountsCmd)
	rootCmd.AddCommand(searchCmd)
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
