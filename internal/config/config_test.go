package config

import (
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
			ID:        "display-1",
			Name:      "Desk monitor",
			InputCode: 15,
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
	if loaded.Device.VendorID != cfg.Device.VendorID || loaded.Monitor.InputCode != cfg.Monitor.InputCode {
		t.Fatalf("loaded config differs: %#v", loaded)
	}
}
