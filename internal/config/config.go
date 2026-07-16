package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const SchemaVersion = 3

type Config struct {
	SchemaVersion  int           `json:"schema_version"`
	Device         DeviceConfig  `json:"device"`
	Monitor        MonitorConfig `json:"monitor"`
	Controller     Controller    `json:"controller"`
	PollIntervalMS int           `json:"poll_interval_ms"`
	DebounceChecks int           `json:"debounce_checks"`
	SwitchOnStart  bool          `json:"switch_on_start"`
	RetrySeconds   int           `json:"retry_seconds"`
}

type DeviceConfig struct {
	Name      string `json:"name"`
	VendorID  uint16 `json:"vendor_id"`
	ProductID uint16 `json:"product_id"`
	Serial    string `json:"serial,omitempty"`
}

type MonitorConfig struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	ConnectedInputCode    uint16 `json:"connected_input_code"`
	DisconnectedInputCode uint16 `json:"disconnected_input_code,omitempty"`
	InputCode             uint16 `json:"input_code,omitempty"`
}

type Controller struct {
	Backend    string `json:"backend"`
	Executable string `json:"executable,omitempty"`
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user configuration directory: %w", err)
	}
	return filepath.Join(dir, "KvmVidSwitcher", "config.json"), nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("configuration not found at %s; run 'kvmvidswitcher setup'", path)
		}
		return Config{}, fmt.Errorf("read configuration: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse configuration: %w", err)
	}
	cfg.migrate()
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	cfg.migrate()
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		return fmt.Errorf("write temporary configuration: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		return fmt.Errorf("replace configuration: %w", err)
	}
	return nil
}

func (cfg *Config) applyDefaults() {
	if cfg.SchemaVersion == 0 {
		cfg.SchemaVersion = SchemaVersion
	}
	if cfg.PollIntervalMS == 0 {
		cfg.PollIntervalMS = 250
	}
	if cfg.DebounceChecks == 0 {
		cfg.DebounceChecks = 2
	}
	if cfg.RetrySeconds == 0 {
		cfg.RetrySeconds = 5
	}
}

func (cfg *Config) migrate() {
	if cfg.SchemaVersion == 1 {
		if cfg.Monitor.ConnectedInputCode == 0 {
			cfg.Monitor.ConnectedInputCode = cfg.Monitor.InputCode
		}
		cfg.Monitor.InputCode = 0
		cfg.SchemaVersion = 2
	}
	if cfg.SchemaVersion == 2 {
		if cfg.PollIntervalMS == 1000 {
			cfg.PollIntervalMS = 250
		}
		cfg.SchemaVersion = SchemaVersion
	}
}

func (cfg Config) InputCodeForDeviceState(present bool) uint16 {
	if present {
		return cfg.Monitor.ConnectedInputCode
	}
	return cfg.Monitor.DisconnectedInputCode
}

func (cfg Config) Validate() error {
	if cfg.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema version %d", cfg.SchemaVersion)
	}
	if cfg.Device.VendorID == 0 || cfg.Device.ProductID == 0 {
		return errors.New("USB vendor_id and product_id must be non-zero")
	}
	if cfg.Monitor.ID == "" {
		return errors.New("monitor id is required")
	}
	if cfg.Monitor.ConnectedInputCode == 0 {
		return errors.New("monitor connected_input_code must be non-zero")
	}
	if cfg.PollIntervalMS < 250 || cfg.PollIntervalMS > 60000 {
		return errors.New("poll_interval_ms must be between 250 and 60000")
	}
	if cfg.DebounceChecks < 1 || cfg.DebounceChecks > 20 {
		return errors.New("debounce_checks must be between 1 and 20")
	}
	if cfg.RetrySeconds < 1 || cfg.RetrySeconds > 300 {
		return errors.New("retry_seconds must be between 1 and 300")
	}
	switch runtime.GOOS {
	case "darwin":
		if cfg.Controller.Backend != "m1ddc" {
			return fmt.Errorf("unsupported macOS controller %q", cfg.Controller.Backend)
		}
		if cfg.Controller.Executable == "" {
			return errors.New("controller executable is required on macOS")
		}
	case "windows":
		if cfg.Controller.Backend != "windows-ddc" {
			return fmt.Errorf("unsupported Windows controller %q", cfg.Controller.Backend)
		}
	}
	return nil
}
