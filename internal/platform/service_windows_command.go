package platform

import (
	"fmt"
	"strings"
)

const maxScheduledTaskCommandLength = 261

func windowsScheduledTaskCommand(launcherPath string) string {
	return fmt.Sprintf(
		`powershell.exe -NoLogo -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File "%s"`,
		launcherPath,
	)
}

func escapePowerShellSingleQuoted(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func taskNotFoundOutput(output string) bool {
	return strings.Contains(strings.ToLower(output), "cannot find")
}
