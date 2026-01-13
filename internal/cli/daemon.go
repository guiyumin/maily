package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// daemon commands are deprecated, redirect to server

var daemonCmd = &cobra.Command{
	Use:        "daemon",
	Short:      "Deprecated: use 'maily server' instead",
	Deprecated: "use 'maily server' instead",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Deprecated: use 'maily server start' instead",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("'maily daemon' is deprecated. Use 'maily server' instead.")
		fmt.Println()
		fmt.Println("Running 'maily server start'...")
		fmt.Println()
		runServer()
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Deprecated: use 'maily server status' instead",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("'maily daemon' is deprecated. Use 'maily server' instead.")
		fmt.Println()
		checkServerStatus()
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Deprecated: use 'maily server stop' instead",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("'maily daemon' is deprecated. Use 'maily server' instead.")
		fmt.Println()
		stopServer()
	},
}

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	rootCmd.AddCommand(daemonCmd)
}
