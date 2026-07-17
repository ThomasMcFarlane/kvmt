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

	"github.com/ThomasMcFarlane/kvmt/internal/config"
	"github.com/ThomasMcFarlane/kvmt/internal/platform"
)

type wizard struct {
	in  *bufio.Reader
	out io.Writer
}

type detectedDevice struct {
	device      platform.USBDevice
	fastTrigger *platform.USBDevice
}

func RunWizard(ctx context.Context, in io.Reader, out io.Writer, path string) error {
	w := wizard{in: bufio.NewReader(in), out: out}
	fmt.Fprintln(out, "KVMT setup")
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
	connectedPrompt := "Monitor input used by this computer when the KVM is connected"
	if runtime.GOOS == "windows" {
		connectedPrompt = "Monitor input used by Windows when the KVM is connected"
	}
	connectedInputCode, err := w.chooseInput(connectedPrompt)
	if err != nil {
		return err
	}
	disconnectedInputCode := uint16(0)
	if runtime.GOOS == "windows" {
		disconnectedInputCode, err = w.chooseInput("Monitor input used by the Mac when the KVM disconnects from Windows")
		if err != nil {
			return err
		}
	}

	detected, err := w.detectKVMDevice(ctx)
	if err != nil {
		return err
	}

	testLabel := "this computer"
	if runtime.GOOS == "windows" {
		testLabel = "Windows"
	}
	fmt.Fprintf(out, "\nTest the %s input %d now? [Y/n]: ", testLabel, connectedInputCode)
	test, err := w.readYesNo(true)
	if err != nil {
		return err
	}
	if test {
		if err := platform.SwitchInput(ctx, controller, monitor.ID, connectedInputCode); err != nil {
			return fmt.Errorf("test monitor input: %w", err)
		}
		fmt.Fprintln(out, "Monitor command succeeded.")
	}
	if disconnectedInputCode != 0 {
		fmt.Fprintf(out, "\nTest the Mac input %d for five seconds, then restore Windows automatically? [Y/n]: ", disconnectedInputCode)
		testAway, err := w.readYesNo(true)
		if err != nil {
			return err
		}
		if testAway {
			fmt.Fprintln(out, "Switching to the Mac input; Windows will restore in five seconds...")
			if err := platform.SwitchInput(ctx, controller, monitor.ID, disconnectedInputCode); err != nil {
				return fmt.Errorf("test Mac monitor input: %w", err)
			}
			time.Sleep(5 * time.Second)
			if err := platform.SwitchInput(ctx, controller, monitor.ID, connectedInputCode); err != nil {
				return fmt.Errorf("restore Windows monitor input after Mac test: %w", err)
			}
			fmt.Fprintln(out, "Both monitor inputs responded successfully.")
		}
	}

	cfg := config.Config{
		SchemaVersion: config.SchemaVersion,
		Device:        deviceConfig(detected),
		Monitor: config.MonitorConfig{
			ID:                    monitor.ID,
			Name:                  monitor.Name,
			ConnectedInputCode:    connectedInputCode,
			DisconnectedInputCode: disconnectedInputCode,
		},
		Controller:      controller,
		PollIntervalMS:  100,
		DebounceChecks:  2,
		SwitchOnStart:   true,
		RetrySeconds:    5,
		RetryIntervalMS: 500,
	}
	if err := config.Save(path, cfg); err != nil {
		return err
	}

	fmt.Fprintf(out, "\nConfiguration saved to %s\n", path)
	if disconnectedInputCode != 0 {
		fmt.Fprintf(out, "KVM connected → input %d; KVM disconnected → input %d.\n", connectedInputCode, disconnectedInputCode)
	}
	fmt.Fprintln(out, "Run 'kvmt switch --state disconnected' to test the Mac input.")
	fmt.Fprintln(out, "Run 'kvmt switch --state connected' to restore Windows.")
	fmt.Fprintln(out, "Run 'kvmt service install' to start it automatically.")
	return nil
}

func RunEditWizard(ctx context.Context, in io.Reader, out io.Writer, path string) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	w := wizard{in: bufio.NewReader(in), out: out}
	fmt.Fprintln(out, "KVMT configuration editor")
	fmt.Fprintf(out, "Loaded %s\n", path)

	for {
		connectedLabel := "This computer input"
		disconnectedLabel := "Disconnected input"
		if runtime.GOOS == "windows" {
			connectedLabel = "Windows input"
			disconnectedLabel = "Mac input"
		}
		fmt.Fprintln(out)
		fmt.Fprintf(out, "  1. KVM USB device — %s (VID:%04X PID:%04X)\n", cfg.Device.Name, cfg.Device.VendorID, cfg.Device.ProductID)
		if cfg.Device.FastTrigger != nil {
			fmt.Fprintf(out, "     Fast trigger — VID:%04X PID:%04X\n", cfg.Device.FastTrigger.VendorID, cfg.Device.FastTrigger.ProductID)
		}
		fmt.Fprintf(out, "  2. Monitor — %s\n", cfg.Monitor.Name)
		fmt.Fprintf(out, "  3. %s — input %d\n", connectedLabel, cfg.Monitor.ConnectedInputCode)
		fmt.Fprintf(out, "  4. %s — input %d\n", disconnectedLabel, cfg.Monitor.DisconnectedInputCode)
		fmt.Fprintln(out, "  5. Test the configured monitor inputs")
		fmt.Fprintln(out, "  6. Save changes")
		fmt.Fprintln(out, "  7. Cancel without saving")
		choice, err := w.readChoice("Select a section", 7)
		if err != nil {
			return err
		}

		switch choice {
		case 0:
			detected, err := w.detectKVMDevice(ctx)
			if err != nil {
				return err
			}
			cfg.Device = deviceConfig(detected)
		case 1:
			monitors, err := platform.ListMonitors(ctx, cfg.Controller)
			if err != nil {
				return fmt.Errorf("discover monitors: %w", err)
			}
			monitor, err := w.chooseMonitor(monitors)
			if err != nil {
				return err
			}
			cfg.Monitor.ID = monitor.ID
			cfg.Monitor.Name = monitor.Name
		case 2:
			code, err := w.chooseInput("Monitor input used when the KVM is connected")
			if err != nil {
				return err
			}
			cfg.Monitor.ConnectedInputCode = code
		case 3:
			code, err := w.chooseInput("Monitor input used when the KVM is disconnected")
			if err != nil {
				return err
			}
			cfg.Monitor.DisconnectedInputCode = code
		case 4:
			if err := w.testInputs(ctx, cfg); err != nil {
				return err
			}
		case 5:
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(out, "\nConfiguration saved to %s\n", path)
			fmt.Fprintln(out, "Run 'kvmt service install' to apply it to the background service.")
			return nil
		case 6:
			fmt.Fprintln(out, "Configuration was not changed.")
			return nil
		}
	}
}

func deviceConfig(detected detectedDevice) config.DeviceConfig {
	device := config.DeviceConfig{
		Name:      detected.device.Name,
		VendorID:  detected.device.VendorID,
		ProductID: detected.device.ProductID,
		Serial:    detected.device.Serial,
	}
	if detected.fastTrigger != nil {
		device.FastTrigger = &config.DeviceMatch{
			Name:      detected.fastTrigger.Name,
			VendorID:  detected.fastTrigger.VendorID,
			ProductID: detected.fastTrigger.ProductID,
		}
	}
	return device
}

func (w wizard) testInputs(ctx context.Context, cfg config.Config) error {
	if cfg.Monitor.DisconnectedInputCode == 0 {
		if err := platform.SwitchInput(ctx, cfg.Controller, cfg.Monitor.ID, cfg.Monitor.ConnectedInputCode); err != nil {
			return fmt.Errorf("test connected monitor input: %w", err)
		}
		fmt.Fprintln(w.out, "The configured monitor input responded successfully.")
		return nil
	}
	if err := platform.SwitchInput(ctx, cfg.Controller, cfg.Monitor.ID, cfg.Monitor.DisconnectedInputCode); err != nil {
		return fmt.Errorf("test disconnected monitor input: %w", err)
	}
	fmt.Fprintln(w.out, "Switched to the disconnected input; restoring in five seconds...")
	time.Sleep(5 * time.Second)
	if err := platform.SwitchInput(ctx, cfg.Controller, cfg.Monitor.ID, cfg.Monitor.ConnectedInputCode); err != nil {
		return fmt.Errorf("restore connected monitor input: %w", err)
	}
	fmt.Fprintln(w.out, "Both monitor inputs responded successfully.")
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

func (w wizard) chooseInput(prompt string) (uint16, error) {
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
	fmt.Fprintf(w.out, "\n%s:\n", prompt)
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

func (w wizard) detectKVMDevice(ctx context.Context) (detectedDevice, error) {
	connected, err := platform.ListUSBDevicesDetailed(ctx)
	if err != nil {
		return detectedDevice{}, fmt.Errorf("list connected USB devices: %w", err)
	}
	connected = uniqueDevices(connected)
	if len(connected) == 0 {
		return detectedDevice{}, fmt.Errorf("no connected USB devices found")
	}
	fastConnected, err := platform.ListFastPresenceDevices(ctx)
	if err != nil {
		fastConnected = nil
	}
	fastConnected = uniqueDevices(fastConnected)

	fmt.Fprintln(w.out, "\nKVM device detection:")
	fmt.Fprintln(w.out, "The wizard can watch which USB device disappears when you switch away.")
	fmt.Fprint(w.out, "Use assisted detection? [Y/n]: ")
	assisted, err := w.readYesNo(true)
	if err != nil {
		return detectedDevice{}, err
	}

	candidates := connected
	var fastCandidates []platform.USBDevice
	if assisted {
		fmt.Fprintln(w.out)
		fmt.Fprintln(w.out, "1. Make sure the KVM is switched to this computer.")
		fmt.Fprintln(w.out, "2. Press Enter, switch the KVM away for three seconds, then switch it back.")
		fmt.Fprintln(w.out, "3. The wizard watches for ten seconds; no keyboard input is needed.")
		if _, err := w.readLine(); err != nil {
			return detectedDevice{}, err
		}
		candidates, fastCandidates, err = observeDisappearingDevices(ctx, connected, fastConnected, 10*time.Second)
		if err != nil {
			return detectedDevice{}, err
		}
		candidates = uniqueDevices(candidates)
		fastCandidates = uniqueDevices(fastCandidates)
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
		return detectedDevice{}, err
	}
	detected := detectedDevice{device: candidates[index]}
	if len(fastCandidates) > 0 {
		sort.SliceStable(fastCandidates, func(i, j int) bool {
			if fastCandidates[i].VendorID != fastCandidates[j].VendorID {
				return fastCandidates[i].VendorID < fastCandidates[j].VendorID
			}
			return fastCandidates[i].ProductID < fastCandidates[j].ProductID
		})
		detected.fastTrigger = &fastCandidates[0]
		fmt.Fprintf(
			w.out,
			"Fast trigger detected automatically — VID:%04X PID:%04X.\n",
			detected.fastTrigger.VendorID,
			detected.fastTrigger.ProductID,
		)
	} else if assisted && runtime.GOOS == "windows" {
		fmt.Fprintln(w.out, "No fast keyboard or mouse trigger was detected; USB hub removal may remain delayed.")
	}
	return detected, nil
}

func observeDisappearingDevices(
	ctx context.Context,
	initial []platform.USBDevice,
	initialFast []platform.USBDevice,
	duration time.Duration,
) ([]platform.USBDevice, []platform.USBDevice, error) {
	initialByKey := devicesByKey(initial)
	initialFastByKey := devicesByKey(initialFast)
	disappeared := make(map[string]platform.USBDevice)
	disappearedFast := make(map[string]platform.USBDevice)
	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-deadline.C:
			return deviceMapValues(disappeared), deviceMapValues(disappearedFast), nil
		case <-ticker.C:
			current, err := platform.ListUSBDevices(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("observe USB devices: %w", err)
			}
			markDisappeared(initialByKey, devicesByKey(current), disappeared)
			currentFast, err := platform.ListFastPresenceDevices(ctx)
			if err == nil {
				markDisappeared(initialFastByKey, devicesByKey(currentFast), disappearedFast)
			}
		}
	}
}

func devicesByKey(devices []platform.USBDevice) map[string]platform.USBDevice {
	result := make(map[string]platform.USBDevice, len(devices))
	for _, device := range devices {
		result[deviceKey(device)] = device
	}
	return result
}

func markDisappeared(
	initial map[string]platform.USBDevice,
	current map[string]platform.USBDevice,
	disappeared map[string]platform.USBDevice,
) {
	for key, device := range initial {
		if _, present := current[key]; !present {
			disappeared[key] = device
		}
	}
}

func deviceMapValues(devices map[string]platform.USBDevice) []platform.USBDevice {
	result := make([]platform.USBDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, device)
	}
	return result
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
	fmt.Fprintf(out, "KVM connected input: %d\n", cfg.Monitor.ConnectedInputCode)
	if cfg.Monitor.DisconnectedInputCode != 0 {
		fmt.Fprintf(out, "KVM disconnected input: %d\n", cfg.Monitor.DisconnectedInputCode)
	}
	if cfg.Device.FastTrigger != nil {
		fmt.Fprintf(
			out,
			"Fast KVM trigger: CONFIGURED (VID:%04X PID:%04X)\n",
			cfg.Device.FastTrigger.VendorID,
			cfg.Device.FastTrigger.ProductID,
		)
	} else if runtime.GOOS == "windows" {
		fmt.Fprintln(out, "Fast KVM trigger: NOT CONFIGURED (run 'kvmt edit')")
	}

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
