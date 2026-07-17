//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const scheduledTaskName = "KvmVidSwitcher"

func InstallService(executable, configPath string) (string, error) {
	_ = exec.Command("schtasks.exe", "/End", "/TN", scheduledTaskName).Run()
	stopOldDaemon := fmt.Sprintf(
		`Get-Process -Name kvmvidswitcher -ErrorAction SilentlyContinue | Where-Object { $_.Id -ne %d } | Stop-Process -Force`,
		os.Getpid(),
	)
	_ = exec.Command(
		"powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Command", stopOldDaemon,
	).Run()
	time.Sleep(300 * time.Millisecond)
	installedExecutable, err := installExecutable(executable)
	if err != nil {
		return "", err
	}
	launcherPath, err := writeWindowsServiceLauncher(installedExecutable, configPath)
	if err != nil {
		return "", err
	}
	command := windowsScheduledTaskCommand(launcherPath)
	if len(command) > maxScheduledTaskCommandLength {
		return "", fmt.Errorf("scheduled task command is %d characters; Windows permits at most %d", len(command), maxScheduledTaskCommandLength)
	}
	output, err := exec.Command(
		"schtasks.exe",
		"/Create",
		"/TN", scheduledTaskName,
		"/SC", "ONLOGON",
		"/TR", command,
		"/RL", "LIMITED",
		"/F",
	).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("create scheduled task: %w: %s", err, strings.TrimSpace(string(output)))
	}
	output, err = exec.Command("schtasks.exe", "/Run", "/TN", scheduledTaskName).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("start scheduled task: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return `Windows Scheduled Task "` + scheduledTaskName + `"`, nil
}

func writeWindowsServiceLauncher(executable, configPath string) (string, error) {
	launcherPath := filepath.Join(filepath.Dir(executable), "service.vbs")
	logPath := filepath.Join(filepath.Dir(configPath), "service.log")
	childCommand := fmt.Sprintf(`"%s" run --config "%s" --log "%s"`, executable, configPath, logPath)
	content := fmt.Sprintf(
		"Option Explicit\r\nDim shell\r\nSet shell = CreateObject(\"WScript.Shell\")\r\nWScript.Quit shell.Run(%s, 0, True)\r\n",
		vbscriptString(childCommand),
	)
	_ = os.Remove(filepath.Join(filepath.Dir(executable), "service.ps1"))
	temp := launcherPath + ".tmp"
	if err := os.WriteFile(temp, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write background launcher: %w", err)
	}
	if err := os.Rename(temp, launcherPath); err != nil {
		return "", fmt.Errorf("replace background launcher: %w", err)
	}
	return launcherPath, nil
}

func UninstallService() error {
	output, err := exec.Command("schtasks.exe", "/Delete", "/TN", scheduledTaskName, "/F").CombinedOutput()
	if err != nil && !taskNotFoundOutput(string(output)) {
		return fmt.Errorf("delete scheduled task: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func ServiceStatus() (string, error) {
	output, err := exec.Command("schtasks.exe", "/Query", "/TN", scheduledTaskName, "/FO", "LIST", "/V").CombinedOutput()
	if err != nil {
		return "not installed", nil
	}
	text := strings.ToLower(string(output))
	switch {
	case strings.Contains(text, "status:") && strings.Contains(text, "running"):
		return "running", nil
	default:
		return "installed", nil
	}
}

func ServiceLogPath(configPath string) (string, error) {
	return filepath.Join(filepath.Dir(configPath), "service.log"), nil
}
