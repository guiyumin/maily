package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"maily/internal/auth"
)

var accountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "List all accounts",
	Run: func(cmd *cobra.Command, args []string) {
		handleAccounts()
	},
}

func handleAccounts() {
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if len(store.Accounts) == 0 {
		fmt.Println("No accounts configured.")
		fmt.Println()
		fmt.Println("Run: maily login gmail")
		return
	}

	fmt.Println()
	fmt.Println("  Accounts:")
	fmt.Println()
	for _, acc := range store.Accounts {
		fmt.Printf("  %s (%s)\n", acc.Credentials.Email, acc.Provider)
	}
	fmt.Println()
}
