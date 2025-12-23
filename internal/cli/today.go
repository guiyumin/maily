package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"maily/internal/auth"
	"maily/internal/calendar"
	"maily/internal/ui"
)

var todayCmd = &cobra.Command{
	Use:     "today",
	Aliases: []string{"t"},
	Short:   "Today's dashboard",
	Long:    `Open a split-panel view showing today's emails and calendar events.`,
	Run: func(cmd *cobra.Command, args []string) {
		runTodayTUI()
	},
}

func runTodayTUI() {
	// Load email accounts
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

	// Check calendar access
	status := calendar.GetAuthStatus()
	switch status {
	case calendar.AuthDenied:
		fmt.Println("Calendar access was denied.")
		fmt.Println()
		fmt.Println("To fix this:")
		fmt.Println("  1. Open System Settings > Privacy & Security > Calendars")
		fmt.Println("  2. Enable access for your terminal app")
		fmt.Println()
		os.Exit(1)
	case calendar.AuthRestricted:
		fmt.Println("Calendar access is restricted by system policy.")
		os.Exit(1)
	case calendar.AuthNotDetermined:
		fmt.Println("Requesting calendar access...")
	}

	calClient, err := calendar.NewClient()
	if err != nil {
		fmt.Printf("Error initializing calendar: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(
		ui.NewTodayApp(store, calClient),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running today view: %v\n", err)
		os.Exit(1)
	}
}
