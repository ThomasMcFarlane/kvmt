//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const scheduledTaskName = "KvmVidSwitcher"

func InstallService(executable, configPath string) (string, error) {
	_ = exec.Command("schtasks.exe", "/End", "/TN", scheduledTaskName).Run()
	time.Sleep(300 * time.Millisecond)
	installedExecutable, err := installExecutable(executable)
	if err != nil {
		return "", err
	}
	command := fmt.Sprintf(
		`powershell.exe -NoLogo -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -Command "& '%s' run --config '%s'"`,
		escapePowerShellSingleQuoted(installedExecutable),
		escapePowerShellSingleQuoted(configPath),
	)
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

func escapePowerShellSingleQuoted(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func UninstallService() error {
	output, err := exec.Command("schtasks.exe", "/Delete", "/TN", scheduledTaskName, "/F").CombinedOutput()
	if err != nil && !strings.Contains(strings.ToLower(string(output)), "cannot find") {
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
