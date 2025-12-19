package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"maily/internal/auth"
)

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

func handleLogoutAccount(email string) {
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if store.RemoveAccount(email) {
		store.Save()
		fmt.Printf("Removed account %s\n", email)
	} else {
		fmt.Printf("Account not found: %s\n", email)
	}
}

func handleLogout() {
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if len(store.Accounts) == 0 {
		fmt.Println("No accounts configured.")
		return
	}

	if len(store.Accounts) == 1 {
		email := store.Accounts[0].Credentials.Email
		store.RemoveAccount(email)
		store.Save()
		fmt.Printf("Removed account %s\n", email)
		return
	}

	fmt.Println()
	fmt.Println("  Which account to remove?")
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
		fmt.Println("Cancelled.")
		return
	}

	if num == 0 {
		fmt.Println("Cancelled.")
		return
	}

	email := store.Accounts[num-1].Credentials.Email
	store.RemoveAccount(email)
	store.Save()
	fmt.Printf("Removed account %s\n", email)
}
