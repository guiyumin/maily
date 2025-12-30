package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func setTempHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestParsePidFile(t *testing.T) {
	setTempHome(t)

	pidFile := getDaemonPidFile()
	if err := os.MkdirAll(filepath.Dir(pidFile), 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	tests := []struct {
		name    string
		content string
		wantPID int
		wantVer string
		wantErr bool
	}{
		{
			name:    "pid_with_version",
			content: "1234:0.6.14",
			wantPID: 1234,
			wantVer: "0.6.14",
		},
		{
			name:    "pid_only",
			content: "5678",
			wantPID: 5678,
			wantVer: "",
		},
		{
			name:    "invalid_pid",
			content: "abc",
			wantErr: true,
		},
	}

	for _, test := range tests {
		if err := os.WriteFile(pidFile, []byte(test.content), 0600); err != nil {
			t.Fatalf("%s: WriteFile error: %v", test.name, err)
		}

		pid, ver, err := parsePidFile()
		if test.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error, got nil", test.name)
			}
			continue
		}

		if err != nil {
			t.Fatalf("%s: parsePidFile error: %v", test.name, err)
		}
		if pid != test.wantPID || ver != test.wantVer {
			t.Fatalf("%s: got pid=%d ver=%q", test.name, pid, ver)
		}
	}
}
