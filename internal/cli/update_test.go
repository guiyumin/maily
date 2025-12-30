package cli

import (
	"os"
	"path/filepath"
	"testing"

	"maily/internal/proc"
)

func TestParseLockInfo(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantPID int
		wantTS  string
		wantErr bool
	}{
		{
			name:    "pid_with_start",
			content: "1234:Wed May 1 00:00:00 2024",
			wantPID: 1234,
			wantTS:  "Wed May 1 00:00:00 2024",
		},
		{
			name:    "pid_only",
			content: "5678",
			wantPID: 5678,
			wantTS:  "",
		},
		{
			name:    "invalid_pid",
			content: "oops",
			wantErr: true,
		},
	}

	for _, test := range tests {
		info, err := proc.ParseLockInfo([]byte(test.content))
		if test.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error", test.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: ParseLockInfo error: %v", test.name, err)
		}
		if info.PID != test.wantPID || info.Start != test.wantTS {
			t.Fatalf("%s: got pid=%d start=%q", test.name, info.PID, info.Start)
		}
	}
}

func TestCleanupStaleLocksRemovesInvalid(t *testing.T) {
	cacheDir := t.TempDir()
	accountDir := filepath.Join(cacheDir, "account")
	if err := os.MkdirAll(accountDir, 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	lockPath := filepath.Join(accountDir, ".sync.lock")
	if err := os.WriteFile(lockPath, []byte("invalid"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	cleanupStaleLocks(cacheDir)

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock to be removed, err=%v", err)
	}
}

func TestCleanupStaleLocksRemovesNonMaily(t *testing.T) {
	cacheDir := t.TempDir()
	accountDir := filepath.Join(cacheDir, "account")
	if err := os.MkdirAll(accountDir, 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	lockPath := filepath.Join(accountDir, ".sync.lock")
	if err := os.WriteFile(lockPath, []byte("1234"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	originalIsMaily := proc.IsMailyProcess
	proc.IsMailyProcess = func(int) bool {
		return false
	}
	defer func() {
		proc.IsMailyProcess = originalIsMaily
	}()

	cleanupStaleLocks(cacheDir)

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock to be removed, err=%v", err)
	}
}

func TestCleanupStaleLocksRemovesMismatchedStart(t *testing.T) {
	cacheDir := t.TempDir()
	accountDir := filepath.Join(cacheDir, "account")
	if err := os.MkdirAll(accountDir, 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	lockPath := filepath.Join(accountDir, ".sync.lock")
	if err := os.WriteFile(lockPath, []byte("1234:bogus"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	originalIsMaily := proc.IsMailyProcess
	originalStart := proc.StartTime
	proc.IsMailyProcess = func(int) bool {
		return true
	}
	proc.StartTime = func(int) (string, error) {
		return "actual", nil
	}
	defer func() {
		proc.IsMailyProcess = originalIsMaily
		proc.StartTime = originalStart
	}()

	cleanupStaleLocks(cacheDir)

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock to be removed, err=%v", err)
	}
}

func TestFindSyncPIDsStartMatch(t *testing.T) {
	cacheDir := t.TempDir()
	account1 := filepath.Join(cacheDir, "account1")
	account2 := filepath.Join(cacheDir, "account2")
	if err := os.MkdirAll(account1, 0700); err != nil {
		t.Fatalf("MkdirAll account1 error: %v", err)
	}
	if err := os.MkdirAll(account2, 0700); err != nil {
		t.Fatalf("MkdirAll account2 error: %v", err)
	}

	if err := os.WriteFile(filepath.Join(account1, ".sync.lock"), []byte("111:actual"), 0600); err != nil {
		t.Fatalf("WriteFile account1 error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(account2, ".sync.lock"), []byte("222:bogus"), 0600); err != nil {
		t.Fatalf("WriteFile account2 error: %v", err)
	}

	originalIsMaily := proc.IsMailyProcess
	originalStart := proc.StartTime
	proc.IsMailyProcess = func(int) bool {
		return true
	}
	proc.StartTime = func(int) (string, error) {
		return "actual", nil
	}
	defer func() {
		proc.IsMailyProcess = originalIsMaily
		proc.StartTime = originalStart
	}()

	pids := findSyncPIDs(cacheDir)

	if len(pids) != 1 || pids[0] != 111 {
		t.Fatalf("expected pid 111 only, got %v", pids)
	}
}
