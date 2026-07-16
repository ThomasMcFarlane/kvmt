package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	t.Parallel()
	controller := Controller{Backend: "windows-ddc"}
	if runtime.GOOS == "darwin" {
		controller = Controller{Backend: "m1ddc", Executable: "/opt/homebrew/bin/m1ddc"}
	}
	cfg := Config{
		SchemaVersion: SchemaVersion,
		Device: DeviceConfig{
			Name:      "KVM receiver",
			VendorID:  0x046d,
			ProductID: 0xc52b,
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
		loaded.Monitor.ConnectedInputCode != cfg.Monitor.ConnectedInputCode ||
		loaded.Monitor.DisconnectedInputCode != cfg.Monitor.DisconnectedInputCode {
		t.Fatalf("loaded config differs: %#v", loaded)
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
}
