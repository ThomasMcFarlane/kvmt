package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ThomasMcFarlane/kvmvidswitcher/internal/config"
	"github.com/ThomasMcFarlane/kvmvidswitcher/internal/platform"
)

type wizard struct {
	in  *bufio.Reader
	out io.Writer
}

func RunWizard(ctx context.Context, in io.Reader, out io.Writer, path string) error {
	w := wizard{in: bufio.NewReader(in), out: out}
	fmt.Fprintln(out, "KvmVidSwitcher setup")
	fmt.Fprintln(out, "This configures the computer you are currently using.")
	fmt.Fprintln(out)

	controller, err := platform.DefaultController()
	if err != nil {
		return err
	}
	monitors, err := platform.ListMonitors(ctx, controller)
	if err != nil {
		return fmt.Errorf("discover monitors: %w", err)
	}
	monitor, err := w.chooseMonitor(monitors)
	if err != nil {
		return err
	}
	inputCode, err := w.chooseInput()
	if err != nil {
		return err
	}

	device, err := w.detectKVMDevice(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\nTest: switch %s to input %d now? [Y/n]: ", monitor.Name, inputCode)
	test, err := w.readYesNo(true)
	if err != nil {
		return err
	}
	if test {
		if err := platform.SwitchInput(ctx, controller, monitor.ID, inputCode); err != nil {
			return fmt.Errorf("test monitor input: %w", err)
		}
		fmt.Fprintln(out, "Monitor command succeeded.")
	}

	cfg := config.Config{
		SchemaVersion: config.SchemaVersion,
		Device: config.DeviceConfig{
			Name:      device.Name,
			VendorID:  device.VendorID,
			ProductID: device.ProductID,
			Serial:    device.Serial,
		},
		Monitor: config.MonitorConfig{
			ID:        monitor.ID,
			Name:      monitor.Name,
			InputCode: inputCode,
		},
		Controller:     controller,
		PollIntervalMS: 1000,
		DebounceChecks: 2,
		SwitchOnStart:  true,
		RetrySeconds:   5,
	}
	if err := config.Save(path, cfg); err != nil {
		return err
	}

	fmt.Fprintf(out, "\nConfiguration saved to %s\n", path)
	fmt.Fprintln(out, "Run 'kvmvidswitcher run --once' to verify detection.")
	fmt.Fprintln(out, "Run 'kvmvidswitcher service install' to start it automatically.")
	return nil
}

func (w wizard) chooseMonitor(monitors []platform.Monitor) (platform.Monitor, error) {
	fmt.Fprintln(w.out, "Controllable monitors:")
	for i, monitor := range monitors {
		fmt.Fprintf(w.out, "  %d. %s (%s)\n", i+1, monitor.Name, monitor.ID)
	}
	index, err := w.readChoice("Select the monitor", len(monitors))
	if err != nil {
		return platform.Monitor{}, err
	}
	return monitors[index], nil
}

func (w wizard) chooseInput() (uint16, error) {
	type inputOption struct {
		name string
		code uint16
	}
	options := []inputOption{
		{name: "DisplayPort 1", code: 15},
		{name: "DisplayPort 2", code: 16},
		{name: "HDMI 1", code: 17},
		{name: "HDMI 2", code: 18},
		{name: "USB-C", code: 27},
		{name: "Custom code", code: 0},
	}
	fmt.Fprintln(w.out, "\nMonitor input used by this computer:")
	for i, option := range options {
		if option.code == 0 {
			fmt.Fprintf(w.out, "  %d. %s\n", i+1, option.name)
		} else {
			fmt.Fprintf(w.out, "  %d. %s (code %d)\n", i+1, option.name, option.code)
		}
	}
	index, err := w.readChoice("Select the input", len(options))
	if err != nil {
		return 0, err
	}
	if options[index].code != 0 {
		return options[index].code, nil
	}
	for {
		fmt.Fprint(w.out, "Enter the monitor's decimal DDC input code (1-65535): ")
		text, err := w.readLine()
		if err != nil {
			return 0, err
		}
		value, err := strconv.ParseUint(text, 0, 16)
		if err == nil && value > 0 {
			return uint16(value), nil
		}
		fmt.Fprintln(w.out, "Enter a number such as 15 or 0x0f.")
	}
}

func (w wizard) detectKVMDevice(ctx context.Context) (platform.USBDevice, error) {
	connected, err := platform.ListUSBDevices(ctx)
	if err != nil {
		return platform.USBDevice{}, fmt.Errorf("list connected USB devices: %w", err)
	}
	connected = uniqueDevices(connected)
	if len(connected) == 0 {
		return platform.USBDevice{}, fmt.Errorf("no connected USB devices found")
	}

	fmt.Fprintln(w.out, "\nKVM device detection:")
	fmt.Fprintln(w.out, "The wizard can watch which USB device disappears when you switch away.")
	fmt.Fprint(w.out, "Use assisted detection? [Y/n]: ")
	assisted, err := w.readYesNo(true)
	if err != nil {
		return platform.USBDevice{}, err
	}

	candidates := connected
	if assisted {
		fmt.Fprintln(w.out)
		fmt.Fprintln(w.out, "1. Make sure the KVM is switched to this computer.")
		fmt.Fprintln(w.out, "2. Press Enter, switch the KVM away for three seconds, then switch it back.")
		fmt.Fprintln(w.out, "3. The wizard watches for ten seconds; no keyboard input is needed.")
		if _, err := w.readLine(); err != nil {
			return platform.USBDevice{}, err
		}
		candidates, err = observeDisappearingDevices(ctx, connected, 10*time.Second)
		if err != nil {
			return platform.USBDevice{}, err
		}
		candidates = uniqueDevices(candidates)
		if len(candidates) == 0 {
			fmt.Fprintln(w.out, "No disappearing device was detected; showing all connected USB devices.")
			candidates = connected
		} else {
			fmt.Fprintln(w.out, "Detected devices that disappeared during the KVM switch:")
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return strings.ToLower(candidates[i].Name) < strings.ToLower(candidates[j].Name)
	})
	fmt.Fprintln(w.out)
	for i, device := range candidates {
		fmt.Fprintf(w.out, "  %d. %s — VID:%04X PID:%04X", i+1, displayDeviceName(device), device.VendorID, device.ProductID)
		if device.Serial != "" {
			fmt.Fprintf(w.out, " — serial %s", device.Serial)
		}
		fmt.Fprintln(w.out)
	}
	index, err := w.readChoice("Select the KVM-connected USB device", len(candidates))
	if err != nil {
		return platform.USBDevice{}, err
	}
	return candidates[index], nil
}

func observeDisappearingDevices(ctx context.Context, initial []platform.USBDevice, duration time.Duration) ([]platform.USBDevice, error) {
	initialByKey := make(map[string]platform.USBDevice)
	for _, device := range initial {
		initialByKey[deviceKey(device)] = device
	}
	disappeared := make(map[string]platform.USBDevice)
	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			result := make([]platform.USBDevice, 0, len(disappeared))
			for _, device := range disappeared {
				result = append(result, device)
			}
			return result, nil
		case <-ticker.C:
			current, err := platform.ListUSBDevices(ctx)
			if err != nil {
				return nil, fmt.Errorf("observe USB devices: %w", err)
			}
			present := make(map[string]bool)
			for _, device := range current {
				present[deviceKey(device)] = true
			}
			for key, device := range initialByKey {
				if !present[key] {
					disappeared[key] = device
				}
			}
		}
	}
}

func uniqueDevices(devices []platform.USBDevice) []platform.USBDevice {
	seen := make(map[string]bool)
	result := make([]platform.USBDevice, 0, len(devices))
	for _, device := range devices {
		key := deviceKey(device)
		if !seen[key] {
			seen[key] = true
			result = append(result, device)
		}
	}
	return result
}

func deviceKey(device platform.USBDevice) string {
	return fmt.Sprintf("%04x:%04x:%s", device.VendorID, device.ProductID, strings.ToLower(device.Serial))
}

func displayDeviceName(device platform.USBDevice) string {
	if strings.TrimSpace(device.Name) == "" {
		return "Unnamed USB device"
	}
	return device.Name
}

func (w wizard) readChoice(prompt string, count int) (int, error) {
	for {
		fmt.Fprintf(w.out, "%s [1-%d]: ", prompt, count)
		line, err := w.readLine()
		if err != nil {
			return 0, err
		}
		value, err := strconv.Atoi(line)
		if err == nil && value >= 1 && value <= count {
			return value - 1, nil
		}
		fmt.Fprintf(w.out, "Enter a number from 1 to %d.\n", count)
	}
}

func (w wizard) readYesNo(defaultYes bool) (bool, error) {
	for {
		line, err := w.readLine()
		if err != nil {
			return false, err
		}
		switch strings.ToLower(line) {
		case "":
			return defaultYes, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprint(w.out, "Enter y or n: ")
		}
	}
}

func (w wizard) readLine() (string, error) {
	line, err := w.in.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	if err == io.EOF && line == "" {
		return "", io.EOF
	}
	return strings.TrimSpace(line), nil
}

func Doctor(ctx context.Context, out io.Writer, path string) error {
	fmt.Fprintf(out, "Operating system: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(out, "Configuration: FAIL (%v)\n", err)
		return nil
	}
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(out, "Configuration: FAIL (%v)\n", err)
		return nil
	}
	fmt.Fprintf(out, "Configuration: OK (%s)\n", path)

	devices, err := platform.ListUSBDevices(ctx)
	if err != nil {
		fmt.Fprintf(out, "USB detection: FAIL (%v)\n", err)
	} else {
		present := false
		for _, device := range devices {
			if platform.Matches(device, cfg.Device) {
				present = true
				break
			}
		}
		if present {
			fmt.Fprintf(out, "KVM device: CONNECTED (%s, VID:%04X PID:%04X)\n", cfg.Device.Name, cfg.Device.VendorID, cfg.Device.ProductID)
		} else {
			fmt.Fprintf(out, "KVM device: NOT CONNECTED (%s, VID:%04X PID:%04X)\n", cfg.Device.Name, cfg.Device.VendorID, cfg.Device.ProductID)
		}
	}

	monitors, err := platform.ListMonitors(ctx, cfg.Controller)
	if err != nil {
		fmt.Fprintf(out, "Monitor controller: FAIL (%v)\n", err)
	} else {
		found := false
		for _, monitor := range monitors {
			if monitor.ID == cfg.Monitor.ID {
				found = true
			}
		}
		if found {
			fmt.Fprintf(out, "Monitor controller: OK (%s)\n", cfg.Monitor.Name)
		} else {
			fmt.Fprintf(out, "Monitor controller: FAIL (configured monitor %s not found)\n", cfg.Monitor.Name)
		}
	}

	status, err := platform.ServiceStatus()
	if err != nil {
		fmt.Fprintf(out, "Background service: UNKNOWN (%v)\n", err)
	} else {
		fmt.Fprintf(out, "Background service: %s\n", status)
	}
	return nil
}
