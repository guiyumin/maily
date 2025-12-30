package proc

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type LockInfo struct {
	PID   int
	Start string
}

// ParseLockInfo parses lock file contents in "PID[:START]" format.
func ParseLockInfo(data []byte) (LockInfo, error) {
	content := strings.TrimSpace(string(data))
	if content == "" {
		return LockInfo{}, fmt.Errorf("empty lock file")
	}

	parts := strings.SplitN(content, ":", 2)
	pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || pid <= 0 {
		return LockInfo{}, fmt.Errorf("invalid PID")
	}

	info := LockInfo{PID: pid}
	if len(parts) == 2 {
		info.Start = strings.TrimSpace(parts[1])
	}

	return info, nil
}

var StartTime = processStartTime

func processStartTime(pid int) (string, error) {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "lstart=")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	start := strings.TrimSpace(string(output))
	if start == "" {
		return "", fmt.Errorf("empty start time")
	}
	return start, nil
}

var IsMailyProcess = isMailyProcess

func isMailyProcess(pid int) bool {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	comm := strings.TrimSpace(string(output))
	return comm == "maily" || strings.HasSuffix(comm, "/maily")
}

var Exists = processExists

func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil || process == nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
