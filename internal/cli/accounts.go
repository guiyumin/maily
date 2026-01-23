package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"maily/internal/auth"
	"maily/internal/i18n"
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
		fmt.Printf("%s: %v\n", i18n.T("common.error"), err)
		os.Exit(1)
	}

	if len(store.Accounts) == 0 {
		fmt.Println(i18n.T("cli.no_accounts"))
		fmt.Println(i18n.T("cli.login_hint"))
		return
	}

	fmt.Println()
	fmt.Println("  " + i18n.T("cli.available_providers"))
	fmt.Println()
	for _, acc := range store.Accounts {
		fmt.Printf("  %s (%s)\n", acc.Credentials.Email, acc.Provider)
	}
	fmt.Println()
}
