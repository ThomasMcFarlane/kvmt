//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const scheduledTaskName = "KVMT"
const legacyScheduledTaskName = "KvmVidSwitcher"

func installedExecutablePath() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("find local application directory: %w", err)
		}
		localAppData = cacheDir
	}
	return filepath.Join(localAppData, "Programs", "kvmt", "bin", "kvmt.exe"), nil
}

func addToUserPath(directory string) (bool, error) {
	script := `$directory=$args[0]; $path=[Environment]::GetEnvironmentVariable('Path','User'); $parts=@($path -split ';' | Where-Object { $_ }); if ($parts -contains $directory) { Write-Output 'present' } else { $newPath=(@($parts)+$directory) -join ';'; [Environment]::SetEnvironmentVariable('Path',$newPath,'User'); Write-Output 'added' }`
	output, err := exec.Command(
		"powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Command", script,
		directory,
	).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("add KVMT to user PATH: %w: %s", err, strings.TrimSpace(string(output)))
	}
	added := strings.Contains(string(output), "added")
	if added {
		broadcastEnvironmentChange()
	}
	return added, nil
}

func broadcastEnvironmentChange() {
	message, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	var result uintptr
	_, _, _ = syscall.NewLazyDLL("user32.dll").NewProc("SendMessageTimeoutW").Call(
		0xffff,
		0x001a,
		0,
		uintptr(unsafe.Pointer(message)),
		0x0002,
		5000,
		uintptr(unsafe.Pointer(&result)),
	)
}

func InstallService(executable, configPath string) (string, error) {
	_ = exec.Command("schtasks.exe", "/End", "/TN", scheduledTaskName).Run()
	_ = exec.Command("schtasks.exe", "/End", "/TN", legacyScheduledTaskName).Run()
	stopOldDaemon := fmt.Sprintf(
		`Get-Process -Name kvmt,kvmvidswitcher -ErrorAction SilentlyContinue | Where-Object { $_.Id -ne %d } | Stop-Process -Force`,
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
	_, _ = exec.Command("schtasks.exe", "/Delete", "/TN", legacyScheduledTaskName, "/F").CombinedOutput()
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
	legacyOutput, legacyErr := exec.Command("schtasks.exe", "/Delete", "/TN", legacyScheduledTaskName, "/F").CombinedOutput()
	if legacyErr != nil && !taskNotFoundOutput(string(legacyOutput)) {
		return fmt.Errorf("delete legacy scheduled task: %w: %s", legacyErr, strings.TrimSpace(string(legacyOutput)))
	}
	return nil
}

func ServiceStatus() (string, error) {
	status, found := windowsTaskStatus(scheduledTaskName)
	if found {
		return status, nil
	}
	legacyStatus, legacyFound := windowsTaskStatus(legacyScheduledTaskName)
	if legacyFound {
		return "legacy " + legacyStatus + " (run 'kvmt service install' to migrate)", nil
	}
	return "not installed", nil
}

func windowsTaskStatus(name string) (string, bool) {
	output, err := exec.Command("schtasks.exe", "/Query", "/TN", name, "/FO", "LIST", "/V").CombinedOutput()
	if err != nil {
		return "", false
	}
	text := strings.ToLower(string(output))
	switch {
	case strings.Contains(text, "status:") && strings.Contains(text, "running"):
		return "running", true
	default:
		return "installed", true
	}
}

func ServiceLogPath(configPath string) (string, error) {
	return filepath.Join(filepath.Dir(configPath), "service.log"), nil
}

func ApplyUpdate(payload []byte, configPath string) (string, error) {
	target, err := installedExecutablePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("create update directory: %w", err)
	}
	if _, err := addToUserPath(filepath.Dir(target)); err != nil {
		return "", err
	}
	staged := target + ".update.exe"
	if err := os.WriteFile(staged, payload, 0o755); err != nil {
		return "", fmt.Errorf("stage update: %w", err)
	}
	scriptPath := filepath.Join(filepath.Dir(target), "apply-update.ps1")
	script := `param([int]$KVMTProcessId,[string]$Source,[string]$Target,[string]$Config,[string]$ScriptPath)
$ErrorActionPreference = 'Stop'
try {
  Wait-Process -Id $KVMTProcessId -ErrorAction SilentlyContinue
  schtasks.exe /End /TN KVMT 2>$null | Out-Null
  schtasks.exe /End /TN KvmVidSwitcher 2>$null | Out-Null
  Get-Process -Name kvmt,kvmvidswitcher -ErrorAction SilentlyContinue | Stop-Process -Force
  Start-Sleep -Milliseconds 300
  Move-Item -LiteralPath $Source -Destination $Target -Force
  if (Test-Path -LiteralPath $Config) {
    & $Target service install --config $Config | Out-Null
  }
} catch {
  $_ | Out-File -FilePath ($Target + '.update-error.log')
} finally {
  Remove-Item -LiteralPath $ScriptPath -Force -ErrorAction SilentlyContinue
}`
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return "", fmt.Errorf("write update helper: %w", err)
	}
	command := exec.Command(
		"powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
		"-KVMTProcessId", fmt.Sprintf("%d", os.Getpid()),
		"-Source", staged,
		"-Target", target,
		"-Config", configPath,
		"-ScriptPath", scriptPath,
	)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := command.Start(); err != nil {
		return "", fmt.Errorf("start update helper: %w", err)
	}
	_ = command.Process.Release()
	return target, nil
}
