package cli

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"maily/internal/auth"
	"maily/internal/cache"
	"maily/internal/client"
	"maily/internal/notify"
	"maily/internal/server"
	"maily/internal/sync"
)

var (
	syncDetach   bool
	syncInternal bool // hidden flag for background process
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync emails from server",
	Long:  "Perform a full sync of emails from the server for all accounts",
	Run: func(cmd *cobra.Command, args []string) {
		if syncDetach && !syncInternal {
			// Re-exec in background with --internal flag
			detachSync()
			return
		}
		runSync()
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVarP(&syncDetach, "detach", "d", false, "Run sync in background and notify when done")
	syncCmd.Flags().BoolVar(&syncInternal, "internal", false, "Internal flag for background sync")
	syncCmd.Flags().MarkHidden("internal")
}

// detachSync starts a background process to do the sync
func detachSync() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Println("Error getting executable path:", err)
		os.Exit(1)
	}

	cmd := exec.Command(exe, "sync", "--internal")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting background sync:", err)
		os.Exit(1)
	}

	fmt.Println("Sync started in background. You'll be notified when complete.")
}

func runSync() {
	// Load accounts
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Println("Error loading accounts:", err)
		notify.Send("Maily Sync", "Error: failed to load accounts")
		os.Exit(1)
	}

	if len(store.Accounts) == 0 {
		fmt.Println("No accounts configured. Run 'maily login' first.")
		notify.Send("Maily Sync", "No accounts configured")
		os.Exit(1)
	}

	// Try to connect to server first
	if cli, err := client.Connect(); err == nil {
		defer cli.Close()
		runSyncViaServer(cli, store)
		return
	}

	// Fall back to direct sync
	runSyncDirect(store)
}

// runSyncViaServer syncs through the running server
func runSyncViaServer(cli *client.Client, store *auth.AccountStore) {
	fmt.Println("Syncing via server...")

	var errors []string
	syncCount := 0
	done := make(chan struct{})

	// Listen for sync events
	go func() {
		events := cli.Events()
		for {
			select {
			case event, ok := <-events:
				if !ok {
					return
				}
				switch event.Type {
				case server.EventSyncCompleted:
					syncCount++
					fmt.Printf("  %s synced\n", event.Account)
					if syncCount >= len(store.Accounts) {
						close(done)
						return
					}
				case server.EventSyncError:
					syncCount++
					errors = append(errors, fmt.Sprintf("%s: %s", event.Account, event.Error))
					fmt.Printf("  %s error: %s\n", event.Account, event.Error)
					if syncCount >= len(store.Accounts) {
						close(done)
						return
					}
				}
			case <-time.After(5 * time.Minute):
				close(done)
				return
			}
		}
	}()

	// Trigger sync for each account
	for i := range store.Accounts {
		account := &store.Accounts[i]
		if err := cli.Sync(account.Credentials.Email, "INBOX"); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", account.Credentials.Email, err.Error()))
			syncCount++
		}
	}

	// Wait for all syncs to complete
	<-done

	// Report results
	if len(errors) > 0 {
		fmt.Printf("Sync completed with %d errors\n", len(errors))
		notify.Send("Maily Sync", fmt.Sprintf("Completed with %d errors", len(errors)))
	} else {
		fmt.Println("Sync complete")
		notify.Send("Maily Sync", fmt.Sprintf("Synced %d accounts", len(store.Accounts)))
	}
}

// runSyncDirect syncs directly via IMAP (fallback when server not running)
func runSyncDirect(store *auth.AccountStore) {
	// Create cache
	c, err := cache.New()
	if err != nil {
		fmt.Println("Error creating cache:", err)
		notify.Send("Maily Sync", "Error: failed to create cache")
		os.Exit(1)
	}

	fmt.Println("Syncing emails (server not running, using direct IMAP)...")

	var errors []string
	for i := range store.Accounts {
		account := &store.Accounts[i]
		fmt.Printf("  Syncing %s...", account.Credentials.Email)

		syncer := sync.NewSyncer(c, account)
		if err := syncer.FullSync("INBOX"); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", account.Credentials.Email, err.Error()))
			fmt.Printf(" error: %v\n", err)
		} else {
			fmt.Println(" done")
		}
	}

	if len(errors) > 0 {
		fmt.Printf("Sync completed with %d errors\n", len(errors))
		notify.Send("Maily Sync", fmt.Sprintf("Completed with %d errors", len(errors)))
	} else {
		fmt.Println("Sync complete")
		notify.Send("Maily Sync", fmt.Sprintf("Synced %d accounts", len(store.Accounts)))
	}
}
