package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"maily/internal/auth"
	"maily/internal/cache"
	"maily/internal/sync"
)

const (
	syncInterval = 30 * time.Minute
	maxLogSize   = 10 * 1024 * 1024 // 10MB
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Background sync daemon",
	Long:  "The daemon syncs your email in the background.",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Run the daemon in foreground (for debugging)",
	Run: func(cmd *cobra.Command, args []string) {
		runDaemon()
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check daemon status and recent logs",
	Run: func(cmd *cobra.Command, args []string) {
		checkDaemonStatus()
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	Run: func(cmd *cobra.Command, args []string) {
		stopDaemon()
	},
}

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	rootCmd.AddCommand(daemonCmd)
}

func getDaemonPidFile() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "maily", "daemon.pid")
}

func getDaemonLogFile() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "maily", "daemon.log")
}

// isDaemonRunning checks if the daemon is currently running
func isDaemonRunning() bool {
	pidFile := getDaemonPidFile()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false
	}

	// Check if process exists and is maily
	if !isMailyProcess(pid) {
		os.Remove(pidFile)
		return false
	}

	return true
}

// isMailyProcess checks if the given PID is a maily process
func isMailyProcess(pid int) bool {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	comm := strings.TrimSpace(string(output))
	return comm == "maily" || strings.HasSuffix(comm, "/maily")
}

// startDaemonBackground starts the daemon in the background
func startDaemonBackground() {
	if isDaemonRunning() {
		return // Already running
	}

	executable, err := os.Executable()
	if err != nil {
		return
	}

	cmd := exec.Command(executable, "daemon", "start")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return
	}
}

// stopDaemon stops the daemon if running
func stopDaemon() {
	pidFile := getDaemonPidFile()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("Daemon is not running.")
		return
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		os.Remove(pidFile)
		fmt.Println("Daemon is not running.")
		return
	}

	if !isMailyProcess(pid) {
		os.Remove(pidFile)
		fmt.Println("Daemon is not running.")
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(pidFile)
		fmt.Println("Daemon is not running.")
		return
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		os.Remove(pidFile)
		fmt.Println("Daemon is not running.")
		return
	}

	// Wait briefly for graceful shutdown
	time.Sleep(500 * time.Millisecond)
	os.Remove(pidFile)
	fmt.Println("Daemon stopped (PID:", pid, ")")
}

func checkDaemonStatus() {
	logFile := getDaemonLogFile()

	if isDaemonRunning() {
		data, _ := os.ReadFile(getDaemonPidFile())
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		fmt.Println("Daemon is running (PID:", pid, ")")
	} else {
		fmt.Println("Daemon is not running")
	}

	fmt.Println("Log file:", logFile)

	// Show recent logs
	logData, err := os.ReadFile(logFile)
	if err != nil {
		fmt.Println("\nNo logs available (log file not found)")
		fmt.Println("Restart the daemon to create the log file: maily daemon stop && maily daemon start")
		return
	}

	if len(logData) == 0 {
		fmt.Println("\nNo logs available (log file is empty)")
		return
	}

	lines := strings.Split(string(logData), "\n")
	start := len(lines) - 10
	if start < 0 {
		start = 0
	}
	fmt.Println()
	fmt.Println("Recent logs:")
	for _, line := range lines[start:] {
		if line != "" {
			fmt.Println(" ", line)
		}
	}
}

func setupLogging(logFile string, alsoToTerminal bool) {
	// Rotate if log exceeds max size
	if info, err := os.Stat(logFile); err == nil && info.Size() > maxLogSize {
		os.Rename(logFile, logFile+".old")
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}

	if alsoToTerminal {
		// Write to both terminal and log file
		mw := io.MultiWriter(os.Stdout, f)
		// Create a pipe to capture writes
		r, w, err := os.Pipe()
		if err != nil {
			os.Stdout = f
			os.Stderr = f
			return
		}
		os.Stdout = w
		os.Stderr = w
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := r.Read(buf)
				if n > 0 {
					mw.Write(buf[:n])
				}
				if err != nil {
					break
				}
			}
		}()
	} else {
		os.Stdout = f
		os.Stderr = f
	}
}

func runDaemon() {
	isTerminal := term.IsTerminal(int(os.Stdin.Fd()))

	// Always set up log file
	logFile := getDaemonLogFile()
	if err := os.MkdirAll(filepath.Dir(logFile), 0700); err == nil {
		setupLogging(logFile, isTerminal)
	}

	// Write PID file
	pidFile := getDaemonPidFile()
	if err := os.MkdirAll(filepath.Dir(pidFile), 0700); err == nil {
		os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0600)
	}
	defer os.Remove(pidFile)

	// Load accounts
	store, err := auth.LoadAccountStore()
	if err != nil {
		fmt.Println("Error loading accounts:", err)
		if isTerminal {
			os.Exit(1)
		}
		return
	}

	if len(store.Accounts) == 0 {
		fmt.Println("No accounts configured. Run 'maily login gmail' first.")
		if isTerminal {
			os.Exit(1)
		}
		return
	}

	// Create cache
	c, err := cache.New()
	if err != nil {
		fmt.Println("Error creating cache:", err)
		if isTerminal {
			os.Exit(1)
		}
		return
	}

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initial sync
	syncAllAccounts(store, c)

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	fmt.Println("Daemon started, syncing every", syncInterval)

	for {
		select {
		case <-ticker.C:
			syncAllAccounts(store, c)
		case sig := <-sigChan:
			fmt.Println("Received signal:", sig)
			return
		}
	}
}

func syncAllAccounts(store *auth.AccountStore, c *cache.Cache) {
	for i := range store.Accounts {
		account := &store.Accounts[i]
		syncer := sync.NewSyncer(c, account)

		if err := syncer.FullSync("INBOX"); err != nil {
			fmt.Printf("Error syncing %s: %v\n", account.Credentials.Email, err)
		} else {
			fmt.Printf("Synced %s\n", account.Credentials.Email)
		}
	}
}
