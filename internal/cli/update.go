package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
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

		// Check if daemon was running and stop it
		daemonWasRunning := isDaemonRunning()
		if daemonWasRunning {
			fmt.Println("Stopping daemon before update...")
			stopDaemon()
		}

		// Stop any running sync
		if err := stopRunningSyncs(); err != nil {
			return err
		}

		// Perform update
		if err := updater.Update(); err != nil {
			return err
		}

		// Restart daemon if it was running
		if daemonWasRunning {
			fmt.Println("Restarting daemon...")
			startDaemonBackground()
		}

		return nil
	},
}

// stopRunningSyncs stops any running sync operations gracefully
func stopRunningSyncs() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil // Can't check, proceed with update
	}

	cacheDir := filepath.Join(homeDir, ".config", "maily", "cache")

	// Find and stop sync processes
	pids := findSyncPIDs(cacheDir)
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
		pids = findSyncPIDs(cacheDir)
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

	// Clean up stale lock files
	time.Sleep(100 * time.Millisecond)
	cleanupStaleLocks(cacheDir)

	return nil
}

// findSyncPIDs returns PIDs of running sync processes
func findSyncPIDs(cacheDir string) []int {
	var pids []int

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		lockPath := filepath.Join(cacheDir, entry.Name(), ".sync.lock")
		data, err := os.ReadFile(lockPath)
		if err != nil {
			continue
		}

		info, err := proc.ParseLockInfo(data)
		if err != nil {
			continue
		}

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

// cleanupStaleLocks removes lock files for dead processes
func cleanupStaleLocks(cacheDir string) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		lockPath := filepath.Join(cacheDir, entry.Name(), ".sync.lock")
		data, err := os.ReadFile(lockPath)
		if err != nil {
			continue
		}

		info, err := proc.ParseLockInfo(data)
		if err != nil {
			os.Remove(lockPath)
			continue
		}

		if !proc.IsMailyProcess(info.PID) {
			os.Remove(lockPath)
			continue
		}

		if info.Start != "" {
			start, err := proc.StartTime(info.PID)
			if err != nil || start == "" {
				continue
			}
			if start != info.Start {
				os.Remove(lockPath)
			}
		}
	}
}
