package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"maily/internal/auth"
	"maily/internal/i18n"
)

var logoutYes bool

var logoutCmd = &cobra.Command{
	Use:   "logout [email]",
	Short: "Remove an account",
	Long:  "Remove an email account. If no email specified, prompts for selection.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			handleLogoutAccount(args[0])
		} else {
			handleLogout()
		}
	},
}

func init() {
	logoutCmd.Flags().BoolVarP(&logoutYes, "yes", "y", false, "Skip confirmation prompt")
}

func confirmLogout(email string) bool {
	// Skip prompt if --yes flag or not a terminal (CI/scripts)
	if logoutYes || !term.IsTerminal(int(os.Stdin.Fd())) {
		return true
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Remove account %s? [y/N]: ", email)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func handleLogoutAccount(email string) {
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Printf("%s: %v\n", i18n.T("common.error"), err)
		os.Exit(1)
	}

	// Check if account exists
	found := false
	for _, acc := range store.Accounts {
		if acc.Credentials.Email == email {
			found = true
			break
		}
	}
	if !found {
		fmt.Printf("%s\n", i18n.T("cli.account_not_found", map[string]any{"Email": email}))
		return
	}

	// Confirm before removing
	if !confirmLogout(email) {
		fmt.Println(i18n.T("common.cancel"))
		return
	}

	if store.RemoveAccount(email) {
		store.Save()
		fmt.Printf("%s\n", i18n.T("cli.logged_out", map[string]any{"Email": email}))
	}
}

func handleLogout() {
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Printf("%s: %v\n", i18n.T("common.error"), err)
		os.Exit(1)
	}

	if len(store.Accounts) == 0 {
		fmt.Println(i18n.T("cli.no_accounts"))
		return
	}

	var email string

	if len(store.Accounts) == 1 {
		email = store.Accounts[0].Credentials.Email
	} else {
		fmt.Println()
		fmt.Println("  " + i18n.T("label.select"))
		fmt.Println()
		for i, acc := range store.Accounts {
			fmt.Printf("  %d. %s\n", i+1, acc.Credentials.Email)
		}
		fmt.Println()

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("  Enter number (or 0 to cancel): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		num, err := strconv.Atoi(input)
		if err != nil || num < 0 || num > len(store.Accounts) {
			fmt.Println(i18n.T("common.cancel"))
			return
		}

		if num == 0 {
			fmt.Println(i18n.T("common.cancel"))
			return
		}

		email = store.Accounts[num-1].Credentials.Email
	}

	// Confirm before removing
	if !confirmLogout(email) {
		fmt.Println(i18n.T("common.cancel"))
		return
	}

	store.RemoveAccount(email)
	store.Save()
	fmt.Printf("%s\n", i18n.T("cli.logged_out", map[string]any{"Email": email}))
}
