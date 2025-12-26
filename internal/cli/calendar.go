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
  maily c add   (prompts for description)`,
	Run: func(cmd *cobra.Command, args []string) {
		var input string
		if len(args) > 0 {
			input = strings.Join(args, " ")
		} else {
			// Prompt for event description
			input = promptForEventDescription()
			if input == "" {
				fmt.Println("Cancelled.")
				return
			}
		}
		runCalendarAdd(input)
	},
}

var calendarListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available calendars",
	Run: func(cmd *cobra.Command, args []string) {
		runCalendarList()
	},
}

func init() {
	calendarAddCmd.Flags().BoolVar(&calendarAddDebug, "debug", false, "Show raw AI response")
	calendarCmd.AddCommand(calendarAddCmd)
	calendarCmd.AddCommand(calendarListCmd)
}

func runCalendarList() {
	client, err := calendar.NewClient()
	if err != nil {
		fmt.Printf("Error accessing calendar: %v\n", err)
		os.Exit(1)
	}

	calendars, err := client.ListCalendars()
	if err != nil {
		fmt.Printf("Error listing calendars: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Available calendars:")
	fmt.Println()
	for _, cal := range calendars {
		fmt.Printf("  %s  %s\n", cal.Color, cal.Title)
	}
	fmt.Println()
	fmt.Println("Use --calendar=\"Calendar Name\" with 'maily c add'")
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

	// Step 1: Show parsed event
	fmt.Println("  ┌─ Parsed Event ─────────────────────────────────┐")
	fmt.Printf("  │  Title:    %-37s│\n", truncate(parsed.Title, 37))
	fmt.Printf("  │  Date:     %-37s│\n", startTime.Format("Monday, Jan 2, 2006"))
	fmt.Printf("  │  Time:     %-37s│\n", fmt.Sprintf("%s - %s", startTime.Format("3:04 PM"), endTime.Format("3:04 PM")))
	if parsed.Location != "" {
		fmt.Printf("  │  Location: %-37s│\n", truncate(parsed.Location, 37))
	}
	fmt.Println("  └────────────────────────────────────────────────┘")

	// Create calendar client to get calendar list
	client, err := calendar.NewClient()
	if err != nil {
		fmt.Printf("Error accessing calendar: %v\n", err)
		os.Exit(1)
	}

	// Get available calendars
	calendars, err := client.ListCalendars()
	if err != nil {
		fmt.Printf("Error listing calendars: %v\n", err)
		os.Exit(1)
	}

	// Step 2: Pick calendar
	fmt.Println()
	calendarID, calendarName, cancelled := promptForCalendar(calendars)
	if cancelled {
		fmt.Println("Cancelled.")
		return
	}

	// Step 3: Handle alarm
	alarmMinutes := parsed.AlarmMinutesBefore
	if !parsed.AlarmSpecified {
		fmt.Println()
		alarmMinutes, cancelled = promptForAlarm()
		if cancelled {
			fmt.Println("Cancelled.")
			return
		}
	}

	// Step 4: Final confirmation with all details
	fmt.Println()
	fmt.Println("  ┌─ Confirm Event ────────────────────────────────┐")
	fmt.Printf("  │  Title:    %-37s│\n", truncate(parsed.Title, 37))
	fmt.Printf("  │  Date:     %-37s│\n", startTime.Format("Monday, Jan 2, 2006"))
	fmt.Printf("  │  Time:     %-37s│\n", fmt.Sprintf("%s - %s", startTime.Format("3:04 PM"), endTime.Format("3:04 PM")))
	if parsed.Location != "" {
		fmt.Printf("  │  Location: %-37s│\n", truncate(parsed.Location, 37))
	}
	fmt.Printf("  │  Calendar: %-37s│\n", truncate(calendarName, 37))
	if alarmMinutes > 0 {
		fmt.Printf("  │  Reminder: %-37s│\n", fmt.Sprintf("%d minutes before", alarmMinutes))
	} else {
		fmt.Printf("  │  Reminder: %-37s│\n", "None")
	}
	fmt.Println("  └────────────────────────────────────────────────┘")

	// Use TUI for confirmation so Esc works
	confirmItems := []SelectorItem{
		{ID: "yes", Label: "Yes, create event"},
		{ID: "no", Label: "No, cancel"},
	}
	confirmID, _, cancelled := RunSelector("Create this event?", confirmItems)
	if cancelled || confirmID == "no" {
		fmt.Println("Cancelled.")
		return
	}

	// Create the event
	event := calendar.Event{
		Title:              parsed.Title,
		StartTime:          startTime,
		EndTime:            endTime,
		Location:           parsed.Location,
		AlarmMinutesBefore: alarmMinutes,
		Calendar:           calendarID,
	}

	eventID, err := client.CreateEvent(event)
	if err != nil {
		fmt.Printf("Error creating event: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("✓ Event created (ID: %s)\n", eventID[:8])
}

func promptForEventDescription() string {
	fmt.Print("Describe your event (e.g., 'tomorrow 9am meeting with Jerry'): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func promptForCalendar(calendars []calendar.Calendar) (string, string, bool) {
	items := make([]SelectorItem, len(calendars))
	for i, cal := range calendars {
		items[i] = SelectorItem{
			ID:    cal.ID,
			Label: cal.Title,
		}
	}

	return RunSelector("Select Calendar", items)
}

func promptForAlarm() (int, bool) {
	items := []SelectorItem{
		{ID: "0", Label: "No reminder"},
		{ID: "5", Label: "5 minutes before"},
		{ID: "10", Label: "10 minutes before"},
		{ID: "15", Label: "15 minutes before"},
		{ID: "30", Label: "30 minutes before"},
		{ID: "60", Label: "1 hour before"},
	}

	id, _, cancelled := RunSelector("Reminder", items)
	if cancelled {
		return 0, true
	}

	minutes, _ := strconv.Atoi(id)
	return minutes, false
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
