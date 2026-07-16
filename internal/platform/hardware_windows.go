//go:build windows

package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ThomasMcFarlane/kvmvidswitcher/internal/config"
)

type windowsUSBDevice struct {
	Name       string `json:"Name"`
	InstanceID string `json:"InstanceId"`
}

func ListUSBDevices(ctx context.Context) ([]USBDevice, error) {
	script := `$items = @(Get-PnpDevice -PresentOnly -ErrorAction Stop | Where-Object { $_.InstanceId -match '^USB\\VID_[0-9A-F]{4}&PID_[0-9A-F]{4}' } | Select-Object @{n='Name';e={$_.FriendlyName}}, InstanceId); $items | ConvertTo-Json -Compress`
	output, err := runPowerShell(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("list USB devices: %w", err)
	}
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}
	var raw json.RawMessage = []byte(output)
	var devices []windowsUSBDevice
	if raw[0] == '[' {
		if err := json.Unmarshal(raw, &devices); err != nil {
			return nil, fmt.Errorf("parse USB device list: %w", err)
		}
	} else {
		var one windowsUSBDevice
		if err := json.Unmarshal(raw, &one); err != nil {
			return nil, fmt.Errorf("parse USB device: %w", err)
		}
		devices = append(devices, one)
	}
	result := make([]USBDevice, 0, len(devices))
	for _, device := range devices {
		upper := strings.ToUpper(device.InstanceID)
		var vendor, product uint64
		fmt.Sscanf(upper, `USB\VID_%04X&PID_%04X`, &vendor, &product)
		parts := strings.Split(device.InstanceID, `\`)
		serial := ""
		if len(parts) > 2 && !strings.Contains(parts[len(parts)-1], "&") {
			serial = parts[len(parts)-1]
		}
		result = append(result, USBDevice{
			Name:      device.Name,
			VendorID:  uint16(vendor),
			ProductID: uint16(product),
			Serial:    serial,
		})
	}
	return result, nil
}

func DefaultController() (config.Controller, error) {
	return config.Controller{Backend: "windows-ddc"}, nil
}

func ListMonitors(ctx context.Context, controller config.Controller) ([]Monitor, error) {
	if controller.Backend != "windows-ddc" {
		return nil, fmt.Errorf("unsupported Windows controller %q", controller.Backend)
	}
	output, err := runPowerShell(ctx, windowsDDCSource+`; [KvmVidSwitcher.NativeDisplay]::List() | ConvertTo-Json -Compress`)
	if err != nil {
		return nil, fmt.Errorf("list physical monitors: %w", err)
	}
	if strings.TrimSpace(output) == "" {
		return nil, fmt.Errorf("Windows found no physical DDC/CI monitors")
	}
	type rawMonitor struct {
		Index int    `json:"Index"`
		Name  string `json:"Name"`
	}
	var raw []rawMonitor
	if strings.HasPrefix(strings.TrimSpace(output), "[") {
		if err := json.Unmarshal([]byte(output), &raw); err != nil {
			return nil, fmt.Errorf("parse monitor list: %w", err)
		}
	} else {
		var one rawMonitor
		if err := json.Unmarshal([]byte(output), &one); err != nil {
			return nil, fmt.Errorf("parse monitor: %w", err)
		}
		raw = append(raw, one)
	}
	monitors := make([]Monitor, 0, len(raw))
	for _, item := range raw {
		monitors = append(monitors, Monitor{ID: strconv.Itoa(item.Index), Name: item.Name})
	}
	return monitors, nil
}

func SwitchInput(ctx context.Context, controller config.Controller, monitorID string, input uint16) error {
	if controller.Backend != "windows-ddc" {
		return fmt.Errorf("unsupported Windows controller %q", controller.Backend)
	}
	index, err := strconv.Atoi(monitorID)
	if err != nil || index < 0 {
		return fmt.Errorf("invalid Windows monitor id %q", monitorID)
	}
	script := fmt.Sprintf(`%s; if (-not [KvmVidSwitcher.NativeDisplay]::SetInput(%d, %d)) { throw 'SetVCPFeature failed. Check that DDC/CI is enabled in the monitor menu.' }`, windowsDDCSource, index, input)
	if _, err := runPowerShell(ctx, script); err != nil {
		return fmt.Errorf("set monitor input: %w", err)
	}
	return nil
}

func runPowerShell(ctx context.Context, script string) (string, error) {
	output, err := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

const windowsDDCSource = `
Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;

namespace KvmVidSwitcher {
    public sealed class PhysicalMonitorInfo {
        public int Index { get; set; }
        public string Name { get; set; }
    }

    public static class NativeDisplay {
        private delegate bool MonitorEnumProc(IntPtr monitor, IntPtr hdc, IntPtr rect, IntPtr data);

        [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
        private struct PHYSICAL_MONITOR {
            public IntPtr Handle;
            [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 128)]
            public string Description;
        }

        [DllImport("user32.dll")]
        private static extern bool EnumDisplayMonitors(IntPtr hdc, IntPtr clip, MonitorEnumProc callback, IntPtr data);

        [DllImport("dxva2.dll", SetLastError = true)]
        private static extern bool GetNumberOfPhysicalMonitorsFromHMONITOR(IntPtr monitor, out uint count);

        [DllImport("dxva2.dll", SetLastError = true)]
        private static extern bool GetPhysicalMonitorsFromHMONITOR(IntPtr monitor, uint size, [Out] PHYSICAL_MONITOR[] physical);

        [DllImport("dxva2.dll", SetLastError = true)]
        private static extern bool DestroyPhysicalMonitors(uint size, PHYSICAL_MONITOR[] physical);

        [DllImport("dxva2.dll", SetLastError = true)]
        private static extern bool SetVCPFeature(IntPtr monitor, byte code, uint value);

        private static List<PHYSICAL_MONITOR> Handles() {
            var result = new List<PHYSICAL_MONITOR>();
            EnumDisplayMonitors(IntPtr.Zero, IntPtr.Zero, delegate(IntPtr monitor, IntPtr hdc, IntPtr rect, IntPtr data) {
                uint count;
                if (!GetNumberOfPhysicalMonitorsFromHMONITOR(monitor, out count) || count == 0) return true;
                var physical = new PHYSICAL_MONITOR[count];
                if (GetPhysicalMonitorsFromHMONITOR(monitor, count, physical)) result.AddRange(physical);
                return true;
            }, IntPtr.Zero);
            return result;
        }

        public static PhysicalMonitorInfo[] List() {
            var handles = Handles();
            var result = new List<PhysicalMonitorInfo>();
            for (int i = 0; i < handles.Count; i++) {
                result.Add(new PhysicalMonitorInfo { Index = i, Name = handles[i].Description });
                var one = new PHYSICAL_MONITOR[] { handles[i] };
                DestroyPhysicalMonitors(1, one);
            }
            return result.ToArray();
        }

        public static bool SetInput(int index, uint value) {
            var handles = Handles();
            bool success = false;
            for (int i = 0; i < handles.Count; i++) {
                if (i == index) success = SetVCPFeature(handles[i].Handle, 0x60, value);
                var one = new PHYSICAL_MONITOR[] { handles[i] };
                DestroyPhysicalMonitors(1, one);
            }
            return success;
        }
    }
}
'@
`
