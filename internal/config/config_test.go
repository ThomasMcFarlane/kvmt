package config

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultPathMigratesLegacyConfiguration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(configDir, "KvmVidSwitcher", "config.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"legacy":true}`)
	if err := os.WriteFile(legacyPath, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(configDir, "kvmt", "config.json"); path != want {
		t.Fatalf("got path %q, want %q", path, want)
	}
	migrated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(migrated, legacy) {
		t.Fatalf("migrated config is %q, want %q", migrated, legacy)
	}
}

func TestSaveAndLoad(t *testing.T) {
	t.Parallel()
	controller := Controller{Backend: "windows-ddc"}
	if runtime.GOOS == "darwin" {
		controller = Controller{Backend: "m1ddc", Executable: "/opt/homebrew/bin/m1ddc"}
	}
	cfg := Config{
		SchemaVersion: SchemaVersion,
		Device: DeviceConfig{
			Name:        "KVM receiver",
			VendorID:    0x046d,
			ProductID:   0xc52b,
			FastTrigger: &DeviceMatch{Name: "Keyboard", VendorID: 0x1234, ProductID: 0x5678},
		},
		Monitor: MonitorConfig{
			ID:                    "display-1",
			Name:                  "Desk monitor",
			ConnectedInputCode:    15,
			DisconnectedInputCode: 17,
		},
		Controller:     controller,
		PollIntervalMS: 1000,
		DebounceChecks: 2,
		SwitchOnStart:  true,
		RetrySeconds:   5,
	}
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Device.VendorID != cfg.Device.VendorID ||
		loaded.Device.FastTrigger == nil ||
		loaded.Device.FastTrigger.ProductID != cfg.Device.FastTrigger.ProductID ||
		loaded.Monitor.ConnectedInputCode != cfg.Monitor.ConnectedInputCode ||
		loaded.Monitor.DisconnectedInputCode != cfg.Monitor.DisconnectedInputCode {
		t.Fatalf("loaded config differs: %#v", loaded)
	}
}

func TestLoadMigratesVersionFourFastTriggerSchema(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.json")
	backend := "windows-ddc"
	executable := ""
	if runtime.GOOS == "darwin" {
		backend = "m1ddc"
		executable = "/opt/homebrew/bin/m1ddc"
	}
	data := []byte(`{
  "schema_version": 4,
  "device": {"name":"KVM","vendor_id":1,"product_id":2},
  "monitor": {"id":"display-1","name":"Monitor","connected_input_code":15,"disconnected_input_code":17},
  "controller": {"backend":"` + backend + `","executable":"` + executable + `"},
  "poll_interval_ms":250,
  "debounce_checks":2,
  "switch_on_start":true,
  "retry_seconds":5,
  "retry_interval_ms":500
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SchemaVersion != SchemaVersion || cfg.Device.FastTrigger != nil || cfg.PollIntervalMS != 100 {
		t.Fatalf("version four config was not migrated: %#v", cfg)
	}
}

func TestLoadMigratesVersionFivePollingSpeed(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.json")
	backend := "windows-ddc"
	executable := ""
	if runtime.GOOS == "darwin" {
		backend = "m1ddc"
		executable = "/opt/homebrew/bin/m1ddc"
	}
	data := []byte(`{
  "schema_version": 5,
  "device": {"name":"KVM","vendor_id":1,"product_id":2,"fast_trigger":{"vendor_id":3,"product_id":4}},
  "monitor": {"id":"display-1","name":"Monitor","connected_input_code":15,"disconnected_input_code":17},
  "controller": {"backend":"` + backend + `","executable":"` + executable + `"},
  "poll_interval_ms":250,
  "debounce_checks":2,
  "switch_on_start":true,
  "retry_seconds":5,
  "retry_interval_ms":500
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SchemaVersion != SchemaVersion || cfg.PollIntervalMS != 100 {
		t.Fatalf("version five config was not migrated: %#v", cfg)
	}
}

func TestLoadMigratesVersionOneInputCode(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.json")
	backend := "windows-ddc"
	executable := ""
	if runtime.GOOS == "darwin" {
		backend = "m1ddc"
		executable = "/opt/homebrew/bin/m1ddc"
	}
	data := []byte(`{
  "schema_version": 1,
  "device": {"name":"KVM","vendor_id":1,"product_id":2},
  "monitor": {"id":"display-1","name":"Monitor","input_code":17},
  "controller": {"backend":"` + backend + `","executable":"` + executable + `"},
  "poll_interval_ms":1000,
  "debounce_checks":2,
  "switch_on_start":true,
  "retry_seconds":5
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SchemaVersion != SchemaVersion || cfg.Monitor.ConnectedInputCode != 17 {
		t.Fatalf("version one config was not migrated: %#v", cfg)
	}
	if cfg.PollIntervalMS != 100 {
		t.Fatalf("migrated poll interval is %d, want 100", cfg.PollIntervalMS)
	}
}

func TestLoadMigratesVersionTwoPollingSpeed(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.json")
	backend := "windows-ddc"
	executable := ""
	if runtime.GOOS == "darwin" {
		backend = "m1ddc"
		executable = "/opt/homebrew/bin/m1ddc"
	}
	data := []byte(`{
  "schema_version": 2,
  "device": {"name":"KVM","vendor_id":1,"product_id":2},
  "monitor": {"id":"display-1","name":"Monitor","connected_input_code":15,"disconnected_input_code":17},
  "controller": {"backend":"` + backend + `","executable":"` + executable + `"},
  "poll_interval_ms":1000,
  "debounce_checks":2,
  "switch_on_start":true,
  "retry_seconds":5
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SchemaVersion != SchemaVersion || cfg.PollIntervalMS != 100 {
		t.Fatalf("version two config was not migrated: %#v", cfg)
	}
	if cfg.RetryIntervalMS != 500 {
		t.Fatalf("migrated retry interval is %d, want 500", cfg.RetryIntervalMS)
	}
}

func TestLoadMigratesVersionThreeRetrySpeed(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.json")
	backend := "windows-ddc"
	executable := ""
	if runtime.GOOS == "darwin" {
		backend = "m1ddc"
		executable = "/opt/homebrew/bin/m1ddc"
	}
	data := []byte(`{
  "schema_version": 3,
  "device": {"name":"KVM","vendor_id":1,"product_id":2},
  "monitor": {"id":"display-1","name":"Monitor","connected_input_code":15,"disconnected_input_code":17},
  "controller": {"backend":"` + backend + `","executable":"` + executable + `"},
  "poll_interval_ms":250,
  "debounce_checks":2,
  "switch_on_start":true,
  "retry_seconds":5
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SchemaVersion != SchemaVersion || cfg.RetryIntervalMS != 500 {
		t.Fatalf("version three config was not migrated: %#v", cfg)
	}
}
