package platform

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsScheduledTaskCommandFitsTaskSchedulerLimit(t *testing.T) {
	t.Parallel()
	launcher := filepath.Join(`C:\Users`, strings.Repeat("a", 80), `AppData\Local\KvmVidSwitcher\bin\service.ps1`)
	command := windowsScheduledTaskCommand(launcher)
	if len(command) > maxScheduledTaskCommandLength {
		t.Fatalf("command is %d characters, limit is %d: %s", len(command), maxScheduledTaskCommandLength, command)
	}
	if !strings.Contains(command, `-File "`) {
		t.Fatalf("command does not use a launcher file: %s", command)
	}
}

func TestEscapePowerShellSingleQuoted(t *testing.T) {
	t.Parallel()
	if got := escapePowerShellSingleQuoted(`C:\Users\O'Brien\app.exe`); got != `C:\Users\O''Brien\app.exe` {
		t.Fatalf("unexpected escaped path %q", got)
	}
}
