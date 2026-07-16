package platform

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsScheduledTaskCommandFitsTaskSchedulerLimit(t *testing.T) {
	t.Parallel()
	launcher := filepath.Join(`C:\Users`, strings.Repeat("a", 80), `AppData\Local\KvmVidSwitcher\bin\service.vbs`)
	command := windowsScheduledTaskCommand(launcher)
	if len(command) > maxScheduledTaskCommandLength {
		t.Fatalf("command is %d characters, limit is %d: %s", len(command), maxScheduledTaskCommandLength, command)
	}
	if !strings.HasPrefix(command, `wscript.exe //B //NoLogo "`) {
		t.Fatalf("command does not use no-console Windows Script Host: %s", command)
	}
}

func TestVBScriptStringQuotesCommand(t *testing.T) {
	t.Parallel()
	command := `"C:\Users\Thomas McFarlane\app.exe" run --config "C:\Users\Thomas McFarlane\config.json"`
	want := `"""C:\Users\Thomas McFarlane\app.exe"" run --config ""C:\Users\Thomas McFarlane\config.json"""`
	if got := vbscriptString(command); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
