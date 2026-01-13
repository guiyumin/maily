package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"maily/internal/cache"
	"maily/internal/proc"
	"maily/internal/updater"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update maily to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if installed via Homebrew
		if executable, err := os.Executable(); err == nil {
			// Resolve symlink to get real path (Homebrew uses symlinks)
			resolved, _ := filepath.EvalSymlinks(executable)
			if strings.Contains(resolved, "/Cellar/") {
				fmt.Println("Maily is installed via Homebrew.")
				fmt.Println("Please run 'brew upgrade maily' instead.")
				return nil
			}
		}

		// Check if server was running and stop it
		serverWasRunning, _, _ := isServerRunning()
		if serverWasRunning {
			fmt.Println("Stopping server before update...")
			stopServer()
		}

		// Stop any running sync
		if err := stopRunningSyncs(); err != nil {
			return err
		}

		// Perform update
		if err := updater.Update(); err != nil {
			return err
		}

		// Restart server if it was running
		if serverWasRunning {
			fmt.Println("Restarting server...")
			startServerBackground()
		}

		return nil
	},
}

// stopRunningSyncs stops any running sync operations gracefully
func stopRunningSyncs() error {
	c, err := cache.New()
	if err != nil {
		return nil // Can't check, proceed with update
	}
	defer c.Close()

	// Find and stop sync processes
	pids := findSyncPIDs(c)
	if len(pids) == 0 {
		return nil
	}

	fmt.Println("Stopping running sync...")

	// Send SIGTERM for graceful shutdown
	for _, pid := range pids {
		if process, err := os.FindProcess(pid); err == nil {
			process.Signal(syscall.SIGTERM)
		}
	}

	// Wait up to 5 seconds for graceful shutdown
	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		pids = findSyncPIDs(c)
		if len(pids) == 0 {
			return nil
		}
	}

	// Force kill remaining processes
	for _, pid := range pids {
		if process, err := os.FindProcess(pid); err == nil {
			process.Signal(syscall.SIGKILL)
		}
	}

	// Clean up stale locks
	time.Sleep(100 * time.Millisecond)
	c.CleanupStaleLocks()

	return nil
}

// findSyncPIDs returns PIDs of running sync processes from SQLite database
func findSyncPIDs(c *cache.Cache) []int {
	var pids []int

	locks, err := c.GetSyncLocks()
	if err != nil {
		return nil
	}

	for _, info := range locks {
		if !proc.IsMailyProcess(info.PID) {
			continue
		}

		if info.Start != "" {
			start, err := proc.StartTime(info.PID)
			if err != nil || start == "" || start != info.Start {
				continue
			}
		}

		pids = append(pids, info.PID)
	}

	return pids
}
