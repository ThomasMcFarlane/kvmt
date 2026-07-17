//go:build darwin

package platform

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

const launchAgentLabel = "com.thomasmcfarlane.kvmt"
const legacyLaunchAgentLabel = "com.thomasmcfarlane.kvmvidswitcher"

func installedExecutablePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".local", "bin", "kvmt"), nil
}

func addToUserPath(directory string) (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("find home directory: %w", err)
	}
	profilePath := filepath.Join(home, ".zprofile")
	line := `export PATH="$HOME/.local/bin:$PATH"`
	data, err := os.ReadFile(profilePath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read shell profile: %w", err)
	}
	if strings.Contains(string(data), line) {
		return false, nil
	}
	file, err := os.OpenFile(profilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return false, fmt.Errorf("open shell profile: %w", err)
	}
	defer file.Close()
	if len(data) > 0 && data[len(data)-1] != '\n' {
		if _, err := file.WriteString("\n"); err != nil {
			return false, fmt.Errorf("update shell profile: %w", err)
		}
	}
	if _, err := fmt.Fprintln(file, line); err != nil {
		return false, fmt.Errorf("update shell profile: %w", err)
	}
	_ = directory
	return true, nil
}

func InstallService(executable, configPath string) (string, error) {
	installedExecutable, err := installExecutable(executable)
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	logDir := filepath.Join(home, "Library", "Logs", "KVMT")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", fmt.Errorf("create log directory: %w", err)
	}
	agentDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return "", fmt.Errorf("create LaunchAgents directory: %w", err)
	}
	plistPath := filepath.Join(agentDir, launchAgentLabel+".plist")
	data := struct {
		Label      string
		Executable string
		Config     string
		Stdout     string
		Stderr     string
	}{
		Label:      launchAgentLabel,
		Executable: installedExecutable,
		Config:     configPath,
		Stdout:     filepath.Join(logDir, "service.log"),
		Stderr:     filepath.Join(logDir, "service-error.log"),
	}
	var rendered bytes.Buffer
	if err := launchAgentTemplate.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("render LaunchAgent: %w", err)
	}
	if err := os.WriteFile(plistPath, rendered.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("write LaunchAgent: %w", err)
	}

	domain := "gui/" + strconv.Itoa(os.Getuid())
	legacyPlist := filepath.Join(agentDir, legacyLaunchAgentLabel+".plist")
	_ = exec.Command("launchctl", "bootout", domain, legacyPlist).Run()
	_ = os.Remove(legacyPlist)
	_ = exec.Command("launchctl", "bootout", domain, plistPath).Run()
	output, err := exec.Command("launchctl", "bootstrap", domain, plistPath).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("start LaunchAgent: %w: %s", err, strings.TrimSpace(string(output)))
	}
	output, err = exec.Command("launchctl", "kickstart", "-k", domain+"/"+launchAgentLabel).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("start KVMT: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return plistPath, nil
}

func UninstallService() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
	legacyPlistPath := filepath.Join(home, "Library", "LaunchAgents", legacyLaunchAgentLabel+".plist")
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain, plistPath).Run()
	_ = exec.Command("launchctl", "bootout", domain, legacyPlistPath).Run()
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove LaunchAgent: %w", err)
	}
	if err := os.Remove(legacyPlistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy LaunchAgent: %w", err)
	}
	return nil
}

func ServiceStatus() (string, error) {
	domain := "gui/" + strconv.Itoa(os.Getuid())
	output, err := exec.Command("launchctl", "print", domain+"/"+launchAgentLabel).CombinedOutput()
	if err != nil {
		legacy, legacyErr := exec.Command("launchctl", "print", domain+"/"+legacyLaunchAgentLabel).CombinedOutput()
		if legacyErr != nil {
			return "not installed", nil
		}
		return "legacy " + launchAgentStatus(string(legacy)) + " (run 'kvmt service install' to migrate)", nil
	}
	return launchAgentStatus(string(output)), nil
}

func launchAgentStatus(text string) string {
	switch {
	case strings.Contains(text, "state = running"):
		return "running"
	case strings.Contains(text, "state ="):
		return "installed (not currently running)"
	default:
		return "installed"
	}
}

func ServiceLogPath(string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Logs", "KVMT", "service.log"), nil
}

func ApplyUpdate(payload []byte, configPath string) (string, error) {
	target, err := installedExecutablePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("create update directory: %w", err)
	}
	temp := target + ".update"
	if err := os.WriteFile(temp, payload, 0o755); err != nil {
		return "", fmt.Errorf("stage update: %w", err)
	}
	if err := os.Chmod(temp, 0o755); err != nil {
		return "", fmt.Errorf("make update executable: %w", err)
	}
	if err := os.Rename(temp, target); err != nil {
		return "", fmt.Errorf("apply update: %w", err)
	}
	if _, err := addToUserPath(filepath.Dir(target)); err != nil {
		return "", err
	}
	if _, err := os.Stat(configPath); err == nil {
		if _, err := InstallService(target, configPath); err != nil {
			return "", fmt.Errorf("restart service after update: %w", err)
		}
	}
	return target, nil
}

var launchAgentTemplate = template.Must(template.New("launch-agent").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>{{.Label | html}}</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{.Executable | html}}</string>
    <string>run</string>
    <string>--config</string>
    <string>{{.Config | html}}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>ProcessType</key>
  <string>Background</string>
  <key>StandardOutPath</key>
  <string>{{.Stdout | html}}</string>
  <key>StandardErrorPath</key>
  <string>{{.Stderr | html}}</string>
</dict>
</plist>
`))
