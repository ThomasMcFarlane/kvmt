package platform

import (
	"fmt"
	"strings"
)

const maxScheduledTaskCommandLength = 261

func windowsScheduledTaskCommand(launcherPath string) string {
	return fmt.Sprintf(
		`wscript.exe //B //NoLogo "%s"`,
		launcherPath,
	)
}

func vbscriptString(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func taskNotFoundOutput(output string) bool {
	return strings.Contains(strings.ToLower(output), "cannot find")
}
