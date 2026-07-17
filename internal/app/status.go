package app

import (
	"fmt"
	"io"

	"github.com/ThomasMcFarlane/kvmt/internal/config"
	"github.com/ThomasMcFarlane/kvmt/internal/platform"
)

func PrintServiceStatus(out io.Writer, configPath string) error {
	status, err := platform.ServiceStatus()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Service: %s\n", status)
	fmt.Fprintf(out, "Configuration: %s\n", configPath)
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(out, "Configuration status: INVALID (%v)\n", err)
		return nil
	}
	fmt.Fprintln(out, "Configuration status: OK")
	fmt.Fprintf(out, "KVM device: %s (VID:%04X PID:%04X)\n", cfg.Device.Name, cfg.Device.VendorID, cfg.Device.ProductID)
	if cfg.Device.FastTrigger != nil {
		fmt.Fprintf(out, "Fast trigger: VID:%04X PID:%04X\n", cfg.Device.FastTrigger.VendorID, cfg.Device.FastTrigger.ProductID)
	} else {
		fmt.Fprintln(out, "Fast trigger: not configured")
	}
	fmt.Fprintf(out, "Monitor: %s (%s)\n", cfg.Monitor.Name, cfg.Monitor.ID)
	fmt.Fprintf(out, "Connected input: %d\n", cfg.Monitor.ConnectedInputCode)
	fmt.Fprintf(out, "Disconnected input: %d\n", cfg.Monitor.DisconnectedInputCode)
	fmt.Fprintf(out, "Polling: %d ms; debounce: %d check(s); retry: %d ms\n", cfg.PollIntervalMS, cfg.DebounceChecks, cfg.RetryIntervalMS)
	return nil
}
