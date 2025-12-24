package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"maily/internal/updater"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update maily to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if daemon is running
		daemonWasRunning := isDaemonRunning()

		// Stop daemon before update
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

// isDaemonRunning checks if the daemon process is currently running
func isDaemonRunning() bool {
	pidFile := getDaemonPidFile()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil || pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false
	}

	return true
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

		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil || pid <= 0 {
			continue
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}

		if err := process.Signal(syscall.Signal(0)); err == nil {
			pids = append(pids, pid)
		}
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

		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil || pid <= 0 {
			os.Remove(lockPath)
			continue
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			os.Remove(lockPath)
			continue
		}

		if err := process.Signal(syscall.Signal(0)); err != nil {
			os.Remove(lockPath)
		}
	}
}

