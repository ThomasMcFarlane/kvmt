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

const launchAgentLabel = "com.thomasmcfarlane.kvmvidswitcher"

func InstallService(executable, configPath string) (string, error) {
	installedExecutable, err := installExecutable(executable)
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	logDir := filepath.Join(home, "Library", "Logs", "KvmVidSwitcher")
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
	_ = exec.Command("launchctl", "bootout", domain, plistPath).Run()
	output, err := exec.Command("launchctl", "bootstrap", domain, plistPath).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("start LaunchAgent: %w: %s", err, strings.TrimSpace(string(output)))
	}
	output, err = exec.Command("launchctl", "kickstart", "-k", domain+"/"+launchAgentLabel).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("start KvmVidSwitcher: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return plistPath, nil
}

func UninstallService() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
	domain := "gui/" + strconv.Itoa(os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain, plistPath).Run()
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove LaunchAgent: %w", err)
	}
	return nil
}

func ServiceStatus() (string, error) {
	domain := "gui/" + strconv.Itoa(os.Getuid())
	output, err := exec.Command("launchctl", "print", domain+"/"+launchAgentLabel).CombinedOutput()
	if err != nil {
		return "not installed", nil
	}
	text := string(output)
	switch {
	case strings.Contains(text, "state = running"):
		return "running", nil
	case strings.Contains(text, "state ="):
		return "installed (not currently running)", nil
	default:
		return "installed", nil
	}
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
