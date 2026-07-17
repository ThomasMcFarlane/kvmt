package platform

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsUserPathCommandHandlesSpacesInDirectory(t *testing.T) {
	t.Parallel()
	directory := `C:\Users\Thomas McFarlane\AppData\Local\Programs\kvmt\bin`
	command := windowsUserPathCommand(directory)

	if strings.Contains(strings.Join(command.Args, " "), directory) {
		t.Fatalf("directory must not be appended to the PowerShell command: %q", command.Args)
	}
	wantEnvironment := userPathDirectoryEnvironment + "=" + directory
	for _, value := range command.Env {
		if value == wantEnvironment {
			return
		}
	}
	t.Fatalf("command environment does not contain %q", wantEnvironment)
}

func TestWindowsScheduledTaskCommandFitsTaskSchedulerLimit(t *testing.T) {
	t.Parallel()
	launcher := filepath.Join(`C:\Users`, strings.Repeat("a", 80), `AppData\Local\KVMT\bin\service.vbs`)
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
	command := `"C:\Users\Example User\app.exe" run --config "C:\Users\Example User\config.json"`
	want := `"""C:\Users\Example User\app.exe"" run --config ""C:\Users\Example User\config.json"""`
	if got := vbscriptString(command); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
