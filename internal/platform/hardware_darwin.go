//go:build darwin

package platform

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ThomasMcFarlane/kvmt/internal/config"
)

func ListUSBDevices(ctx context.Context) ([]USBDevice, error) {
	output, err := exec.CommandContext(ctx, "/usr/sbin/ioreg", "-l", "-p", "IOUSB", "-w", "0", "-r", "-c", "IOUSBHostDevice").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list USB devices with ioreg: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return parseDarwinIOReg(string(output)), nil
}

func ListUSBDevicesDetailed(ctx context.Context) ([]USBDevice, error) {
	return ListUSBDevices(ctx)
}

func fastDevicePresent(context.Context, config.DeviceConfig) (bool, bool, error) {
	return false, false, nil
}

func listFastPresenceDevices(context.Context) ([]USBDevice, error) {
	return nil, nil
}

func DefaultController() (config.Controller, error) {
	path, err := exec.LookPath("m1ddc")
	if err != nil {
		for _, candidate := range []string{"/opt/homebrew/bin/m1ddc", "/usr/local/bin/m1ddc"} {
			if resolved, resolveErr := filepath.EvalSymlinks(candidate); resolveErr == nil {
				return config.Controller{Backend: "m1ddc", Executable: resolved}, nil
			}
		}
		return config.Controller{}, fmt.Errorf("m1ddc is not installed; run 'brew install m1ddc'")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return config.Controller{}, fmt.Errorf("resolve m1ddc path: %w", err)
	}
	return config.Controller{Backend: "m1ddc", Executable: path}, nil
}

func ListMonitors(ctx context.Context, controller config.Controller) ([]Monitor, error) {
	if controller.Backend != "m1ddc" {
		return nil, fmt.Errorf("unsupported macOS controller %q", controller.Backend)
	}
	output, err := exec.CommandContext(ctx, controller.Executable, "display", "list").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list displays with m1ddc: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return parseM1DDCDisplays(string(output))
}

func SwitchInput(ctx context.Context, controller config.Controller, monitorID string, input uint16) error {
	if controller.Backend != "m1ddc" {
		return fmt.Errorf("unsupported macOS controller %q", controller.Backend)
	}
	output, err := exec.CommandContext(ctx, controller.Executable, "display", monitorID, "set", "input", strconv.Itoa(int(input))).CombinedOutput()
	if err != nil {
		return fmt.Errorf("set display input with m1ddc: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
