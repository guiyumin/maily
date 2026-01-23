package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"maily/internal/client"
	"maily/internal/proc"
	"maily/internal/server"
	"maily/internal/version"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Maily server (replaces daemon)",
	Long:  "The server runs in the background and handles email sync, caching, and IMAP operations.",
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the server (foreground for debugging)",
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check server status",
	Run: func(cmd *cobra.Command, args []string) {
		checkServerStatus()
	},
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the server",
	Run: func(cmd *cobra.Command, args []string) {
		stopServer()
	},
}

func init() {
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStatusCmd)
	serverCmd.AddCommand(serverStopCmd)
	rootCmd.AddCommand(serverCmd)
}

// runServer starts the server in foreground
func runServer() {
	if server.IsServerRunning() {
		fmt.Println("Server is already running.")
		fmt.Println("Use 'maily server stop' to stop it first.")
		os.Exit(1)
	}

	srv, err := server.New()
	if err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}

	if err := srv.Run(); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}

// isServerRunning checks if the server is running by PID file
func isServerRunning() (bool, int, string) {
	pidPath := server.GetPidPath()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false, 0, ""
	}

	content := strings.TrimSpace(string(data))
	parts := strings.SplitN(content, ":", 2)

	pid, err := strconv.Atoi(parts[0])
	if err != nil || pid <= 0 {
		return false, 0, ""
	}

	ver := ""
	if len(parts) == 2 {
		ver = parts[1]
	}

	if !proc.IsMailyProcess(pid) {
		os.Remove(pidPath)
		return false, 0, ""
	}

	return true, pid, ver
}

// checkServerStatus shows server status and info
func checkServerStatus() {
	running, pid, ver := isServerRunning()

	if running {
		fmt.Printf("Server is running (PID: %d", pid)
		if ver != "" {
			fmt.Printf(", version: %s", ver)
		}
		fmt.Println(")")
		fmt.Printf("Socket: %s\n", server.GetSocketPath())

		// Try to get more info via client connection
		c, err := client.Connect()
		if err == nil {
			defer c.Close()
			accounts, err := c.GetAccounts()
			if err == nil {
				fmt.Printf("\nAccounts (%d):\n", len(accounts))
				for _, acc := range accounts {
					status := "idle"
					if acc.Syncing {
						status = "syncing"
					}
					fmt.Printf("  %s (%s) - %d emails, %s\n",
						acc.Email, acc.Provider, acc.EmailCount, status)
					if !acc.LastSync.IsZero() {
						fmt.Printf("    Last sync: %s\n", acc.LastSync.Format(time.RFC1123))
					}
				}
			}
		}
	} else {
		fmt.Println("Server is not running")
		fmt.Printf("Socket path: %s\n", server.GetSocketPath())
	}
}

// stopServer stops the running server
func stopServer() {
	running, pid, _ := isServerRunning()

	if !running {
		fmt.Println("Server is not running.")
		return
	}

	// Try graceful shutdown via client
	c, err := client.Connect()
	if err == nil {
		c.Shutdown()
		c.Close()
		time.Sleep(500 * time.Millisecond)
	}

	// Check if still running, force kill if needed
	if stillRunning, _, _ := isServerRunning(); stillRunning {
		process, err := os.FindProcess(pid)
		if err == nil {
			process.Signal(syscall.SIGTERM)
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Clean up
	os.Remove(server.GetPidPath())
	os.Remove(server.GetSocketPath())

	fmt.Printf("Server stopped (PID: %d)\n", pid)
}

// startServerBackground starts the server in background
func startServerBackground() error {
	// Check for version mismatch FIRST (before checking if server is running)
	running, pid, serverVer := isServerRunning()
	if running && serverVer != version.Version {
		// Version mismatch - stop old server so we can start our version
		if process, err := os.FindProcess(pid); err == nil {
			process.Signal(syscall.SIGTERM)
			// Wait for graceful shutdown
			for i := 0; i < 20; i++ { // 2 seconds max
				time.Sleep(100 * time.Millisecond)
				if !server.IsServerRunning() {
					break
				}
			}
		}
		os.Remove(server.GetPidPath())
		os.Remove(server.GetSocketPath())
	}

	// Now check if a compatible server is already running
	if server.IsServerRunning() {
		return nil // Already running with matching version
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}

	// Ensure log directory exists
	homeDir, _ := os.UserHomeDir()
	logDir := filepath.Join(homeDir, ".config", "maily")
	os.MkdirAll(logDir, 0700)

	// Start server process
	cmd := exec.Command(executable, "server", "start")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Wait for socket to be ready
	sockPath := server.GetSocketPath()
	for i := 0; i < 50; i++ { // 5 seconds max
		time.Sleep(100 * time.Millisecond)
		if _, err := os.Stat(sockPath); err == nil {
			return nil
		}
	}

	return fmt.Errorf("server failed to start within timeout")
}
