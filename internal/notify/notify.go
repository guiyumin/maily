package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Send sends a system notification with the given title and message.
// On macOS, uses osascript. On Linux, uses notify-send if available.
func Send(title, message string) error {
	switch runtime.GOOS {
	case "darwin":
		return sendMacOS(title, message)
	case "linux":
		return sendLinux(title, message)
	default:
		// Unsupported platform, silently ignore
		return nil
	}
}

func sendMacOS(title, message string) error {
	script := fmt.Sprintf(`display notification %q with title %q`, message, title)
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

func sendLinux(title, message string) error {
	// Try notify-send (common on most Linux distros)
	cmd := exec.Command("notify-send", title, message)
	return cmd.Run()
}
