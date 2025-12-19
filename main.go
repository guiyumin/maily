package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"cocomail/internal/ui"
)

func main() {
	p := tea.NewProgram(
		ui.NewApp(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
