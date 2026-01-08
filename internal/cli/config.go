package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure maily settings",
	Long:  `Open interactive configuration to view and edit maily settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := RunConfigTUI(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}
