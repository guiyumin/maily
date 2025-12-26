package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"maily/internal/ai"
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

var calendarAddDebug bool

var calendarAddCmd = &cobra.Command{
	Use:   "add [natural language description]",
	Short: "Add event using natural language",
	Long: `Add a calendar event using natural language.

Examples:
  maily c add "tomorrow 9am meeting with Jerry"
  maily c add "lunch with Sarah next Friday at noon"
  maily c add "team standup Monday 10am remind me 5 min before"`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		input := strings.Join(args, " ")
		runCalendarAdd(input)
	},
}

func init() {
	calendarAddCmd.Flags().BoolVar(&calendarAddDebug, "debug", false, "Show raw AI response")
	calendarCmd.AddCommand(calendarAddCmd)
}

func runCalendarAdd(input string) {
	// Check AI availability
	aiClient := ai.NewClient()
	if !aiClient.Available() {
		fmt.Println("Error: No AI CLI found.")
		fmt.Println()
		fmt.Println("Install one of the following:")
		fmt.Println("  - claude (Claude Code)")
		fmt.Println("  - codex  (Codex CLI)")
		fmt.Println("  - gemini (Gemini CLI)")
		fmt.Println("  - ollama (Ollama)")
		os.Exit(1)
	}

	fmt.Printf("Using %s to parse: %q\n", aiClient.Provider(), input)
	fmt.Println()

	// Parse natural language using AI
	prompt := ai.ParseCalendarEventPrompt(input, time.Now())
	response, err := aiClient.Call(prompt)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Debug: show raw AI response
	if calendarAddDebug {
		fmt.Printf("AI response:\n%s\n\n", response)
	}

	// Parse the JSON response
	parsed, err := ai.ParseEventResponse(response)
	if err != nil {
		fmt.Printf("Error parsing AI response: %v\n", err)
		os.Exit(1)
	}

	startTime, err := parsed.GetStartTime()
	if err != nil {
		fmt.Printf("Error parsing start time: %v\n", err)
		os.Exit(1)
	}

	endTime, err := parsed.GetEndTime()
	if err != nil {
		fmt.Printf("Error parsing end time: %v\n", err)
		os.Exit(1)
	}

	// Show parsed event for confirmation
	fmt.Println("┌─ Parsed Event ─────────────────────────────────┐")
	fmt.Printf("│  Title:    %-37s│\n", truncate(parsed.Title, 37))
	fmt.Printf("│  Date:     %-37s│\n", startTime.Format("Monday, Jan 2, 2006"))
	fmt.Printf("│  Time:     %-37s│\n", fmt.Sprintf("%s - %s", startTime.Format("3:04 PM"), endTime.Format("3:04 PM")))
	if parsed.Location != "" {
		fmt.Printf("│  Location: %-37s│\n", truncate(parsed.Location, 37))
	}

	// Handle alarm
	alarmMinutes := parsed.AlarmMinutesBefore
	if !parsed.AlarmSpecified {
		fmt.Println("│                                                │")
		fmt.Println("│  Reminder not specified.                       │")
		fmt.Println("└────────────────────────────────────────────────┘")
		fmt.Println()
		alarmMinutes = promptForAlarm()
	} else {
		if alarmMinutes > 0 {
			fmt.Printf("│  Reminder: %-37s│\n", fmt.Sprintf("%d minutes before", alarmMinutes))
		} else {
			fmt.Printf("│  Reminder: %-37s│\n", "None")
		}
		fmt.Println("└────────────────────────────────────────────────┘")
	}

	fmt.Println()

	// Confirm before creating
	fmt.Print("Create this event? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "" && confirm != "y" && confirm != "yes" {
		fmt.Println("Cancelled.")
		return
	}

	// Create calendar client
	client, err := calendar.NewClient()
	if err != nil {
		fmt.Printf("Error accessing calendar: %v\n", err)
		os.Exit(1)
	}

	// Create the event
	event := calendar.Event{
		Title:              parsed.Title,
		StartTime:          startTime,
		EndTime:            endTime,
		Location:           parsed.Location,
		AlarmMinutesBefore: alarmMinutes,
	}

	eventID, err := client.CreateEvent(event)
	if err != nil {
		fmt.Printf("Error creating event: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("✓ Event created (ID: %s)\n", eventID[:8])
}

func promptForAlarm() int {
	fmt.Print("How many minutes before to remind you? (0 = no reminder): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return 0
	}

	minutes, err := strconv.Atoi(input)
	if err != nil || minutes < 0 {
		fmt.Println("Invalid input, using no reminder.")
		return 0
	}

	return minutes
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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
