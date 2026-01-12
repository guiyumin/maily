package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"maily/config"
	"maily/internal/auth"
	"maily/internal/ui"
)

var rootCmd = &cobra.Command{
	Use:   "maily",
	Short: "A handy CLI email client in your terminal",
	Long:  "maily - A handy CLI email client in your terminal",
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
	rootCmd.AddCommand(calendarCmd)
	rootCmd.AddCommand(todayCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(configCmd)
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
		fmt.Println("  maily login")
		fmt.Println()
		os.Exit(1)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Auto-start daemon if not running
	startDaemonBackground()

	p := tea.NewProgram(
		ui.NewApp(store, &cfg),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	m, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	// Check if we should launch config TUI (e.g., for AI setup)
	if app, ok := m.(ui.App); ok && app.LaunchConfigUI {
		if err := RunConfigTUI(); err != nil {
			fmt.Printf("Error running config: %v\n", err)
			os.Exit(1)
		}
	}
}
