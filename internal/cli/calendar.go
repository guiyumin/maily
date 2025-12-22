package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"maily/internal/calendar"
	"maily/internal/ui"
)

var calendarCmd = &cobra.Command{
	Use:     "calendar",
	Aliases: []string{"c"},
	Short:   "Open calendar",
	Long:    `Open the calendar TUI to view and manage events.`,
	Run: func(cmd *cobra.Command, args []string) {
		runCalendarTUI()
	},
}

func runCalendarTUI() {
	// Check calendar access first
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

	client, err := calendar.NewClient()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(
		ui.NewCalendarApp(client),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running calendar: %v\n", err)
		os.Exit(1)
	}
}
