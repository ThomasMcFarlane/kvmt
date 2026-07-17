package app

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/ThomasMcFarlane/kvmt/internal/config"
)

func TestEditWizardChangesOnlySelectedInput(t *testing.T) {
	t.Parallel()
	controller := config.Controller{Backend: "windows-ddc"}
	if runtime.GOOS == "darwin" {
		controller = config.Controller{Backend: "m1ddc", Executable: "/opt/homebrew/bin/m1ddc"}
	}
	cfg := config.Config{
		SchemaVersion: config.SchemaVersion,
		Device: config.DeviceConfig{
			Name:        "Existing hub",
			VendorID:    0x05e3,
			ProductID:   0x0610,
			FastTrigger: &config.DeviceMatch{Name: "Keyboard", VendorID: 0x1234, ProductID: 0x5678},
		},
		Monitor: config.MonitorConfig{
			ID:                    "monitor-one",
			Name:                  "Existing monitor",
			ConnectedInputCode:    15,
			DisconnectedInputCode: 17,
		},
		Controller:      controller,
		PollIntervalMS:  250,
		DebounceChecks:  2,
		SwitchOnStart:   true,
		RetrySeconds:    5,
		RetryIntervalMS: 500,
	}
	path := t.TempDir() + "/config.json"
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := RunEditWizard(t.Context(), strings.NewReader("3\n4\n6\n"), &output, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Monitor.ConnectedInputCode != 18 {
		t.Fatalf("connected input is %d, want 18", loaded.Monitor.ConnectedInputCode)
	}
	if loaded.Monitor.DisconnectedInputCode != cfg.Monitor.DisconnectedInputCode ||
		loaded.Monitor.ID != cfg.Monitor.ID ||
		loaded.Device.VendorID != cfg.Device.VendorID ||
		loaded.Device.FastTrigger == nil ||
		loaded.Device.FastTrigger.ProductID != cfg.Device.FastTrigger.ProductID {
		t.Fatalf("unselected configuration changed: %#v", loaded)
	}
}
