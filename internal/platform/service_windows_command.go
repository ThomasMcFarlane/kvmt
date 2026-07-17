package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const maxScheduledTaskCommandLength = 261

const userPathDirectoryEnvironment = "KVMT_INSTALL_DIRECTORY"

const userPathPowerShellScript = `$directory=[Environment]::GetEnvironmentVariable('KVMT_INSTALL_DIRECTORY','Process'); $path=[Environment]::GetEnvironmentVariable('Path','User'); $parts=@($path -split ';' | Where-Object { $_ }); if ($parts -contains $directory) { Write-Output 'present' } else { $newPath=(@($parts)+$directory) -join ';'; [Environment]::SetEnvironmentVariable('Path',$newPath,'User'); Write-Output 'added' }`

func windowsUserPathCommand(directory string) *exec.Cmd {
	command := exec.Command(
		"powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Command", userPathPowerShellScript,
	)
	command.Env = append(os.Environ(), userPathDirectoryEnvironment+"="+directory)
	return command
}

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
