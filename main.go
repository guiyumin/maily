package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"cocomail/internal/auth"
	"cocomail/internal/ui"
)

func main() {
	creds, err := auth.LoadCredentials()
	if err != nil {
		fmt.Printf("Error loading credentials: %v\n", err)
		os.Exit(1)
	}

	if creds == nil {
		creds, err = auth.PromptCredentials()
		if err != nil {
			fmt.Printf("Error getting credentials: %v\n", err)
			os.Exit(1)
		}

		if err := auth.SaveCredentials(creds); err != nil {
			fmt.Printf("Error saving credentials: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\nCredentials saved! Starting cocomail...\n")
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
