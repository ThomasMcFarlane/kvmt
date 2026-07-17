//go:build windows

package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ThomasMcFarlane/kvmvidswitcher/internal/config"
)

const (
	cmGetIDListFilterEnumerator = 0x00000001
	cmGetIDListFilterPresent    = 0x00000100
	crSuccess                   = 0
)

var (
	cfgMgr32               = syscall.NewLazyDLL("cfgmgr32.dll")
	user32                 = syscall.NewLazyDLL("user32.dll")
	dxva2                  = syscall.NewLazyDLL("dxva2.dll")
	cmGetDeviceIDListSizeW = cfgMgr32.NewProc("CM_Get_Device_ID_List_SizeW")
	cmGetDeviceIDListW     = cfgMgr32.NewProc("CM_Get_Device_ID_ListW")
	enumDisplayMonitors    = user32.NewProc("EnumDisplayMonitors")
	getPhysicalCount       = dxva2.NewProc("GetNumberOfPhysicalMonitorsFromHMONITOR")
	getPhysicalMonitors    = dxva2.NewProc("GetPhysicalMonitorsFromHMONITOR")
	destroyPhysical        = dxva2.NewProc("DestroyPhysicalMonitor")
	setVCPFeature          = dxva2.NewProc("SetVCPFeature")
	getRawInputDeviceList  = user32.NewProc("GetRawInputDeviceList")
	getRawInputDeviceInfoW = user32.NewProc("GetRawInputDeviceInfoW")
	rawInputCapable        sync.Map
)

type windowsUSBDevice struct {
	Name       string `json:"Name"`
	InstanceID string `json:"InstanceId"`
}

type physicalMonitor struct {
	Handle      uintptr
	Description [128]uint16
}

type rawInputDeviceList struct {
	Device uintptr
	Type   uint32
	_      uint32
}

func ListUSBDevices(ctx context.Context) ([]USBDevice, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	filter, err := syscall.UTF16PtrFromString("USB")
	if err != nil {
		return nil, fmt.Errorf("encode USB enumerator: %w", err)
	}
	flags := uintptr(cmGetIDListFilterEnumerator | cmGetIDListFilterPresent)
	var length uint32
	result, _, _ := cmGetDeviceIDListSizeW.Call(
		uintptr(unsafe.Pointer(&length)),
		uintptr(unsafe.Pointer(filter)),
		flags,
	)
	if result != crSuccess {
		return nil, fmt.Errorf("CM_Get_Device_ID_List_SizeW failed with code 0x%X", result)
	}
	if length <= 1 {
		return nil, nil
	}
	buffer := make([]uint16, length)
	result, _, _ = cmGetDeviceIDListW.Call(
		uintptr(unsafe.Pointer(filter)),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(length),
		flags,
	)
	if result != crSuccess {
		return nil, fmt.Errorf("CM_Get_Device_ID_ListW failed with code 0x%X", result)
	}
	return parseWindowsUSBInstanceIDs(splitUTF16MultiString(buffer)), nil
}

func ListUSBDevicesDetailed(ctx context.Context) ([]USBDevice, error) {
	fast, err := ListUSBDevices(ctx)
	if err != nil {
		return nil, err
	}
	script := `$items = @(Get-PnpDevice -PresentOnly -ErrorAction Stop | Where-Object { $_.InstanceId -match '^USB\\VID_[0-9A-F]{4}&PID_[0-9A-F]{4}' } | Select-Object @{n='Name';e={$_.FriendlyName}}, InstanceId); $items | ConvertTo-Json -Compress`
	output, err := runPowerShell(ctx, script)
	if err != nil || strings.TrimSpace(output) == "" {
		return fast, nil
	}
	var details []windowsUSBDevice
	if strings.HasPrefix(strings.TrimSpace(output), "[") {
		if err := json.Unmarshal([]byte(output), &details); err != nil {
			return fast, nil
		}
	} else {
		var one windowsUSBDevice
		if err := json.Unmarshal([]byte(output), &one); err != nil {
			return fast, nil
		}
		details = append(details, one)
	}
	names := make(map[string]string)
	for _, detail := range details {
		names[strings.ToUpper(detail.InstanceID)] = detail.Name
	}
	for i := range fast {
		for instanceID, name := range names {
			parsed := parseWindowsUSBInstanceIDs([]string{instanceID})
			if len(parsed) == 1 &&
				parsed[0].VendorID == fast[i].VendorID &&
				parsed[0].ProductID == fast[i].ProductID &&
				parsed[0].Serial == fast[i].Serial &&
				strings.TrimSpace(name) != "" {
				fast[i].Name = name
				break
			}
		}
	}
	return fast, nil
}

func fastDevicePresent(ctx context.Context, match config.DeviceConfig) (bool, bool, error) {
	rawDevices, err := listRawInputUSBDevices(ctx)
	if err != nil {
		return false, false, nil
	}
	vendorID := match.VendorID
	productID := match.ProductID
	explicit := match.FastTrigger != nil
	if explicit {
		vendorID = match.FastTrigger.VendorID
		productID = match.FastTrigger.ProductID
	}
	key := fmt.Sprintf("%04X:%04X", vendorID, productID)
	for _, device := range rawDevices {
		if device.VendorID == vendorID && device.ProductID == productID {
			rawInputCapable.Store(key, true)
			return true, true, nil
		}
	}
	if explicit {
		return false, true, nil
	}
	if _, learned := rawInputCapable.Load(key); learned {
		return false, true, nil
	}
	return false, false, nil
}

func listFastPresenceDevices(ctx context.Context) ([]USBDevice, error) {
	return listRawInputUSBDevices(ctx)
}

func listRawInputUSBDevices(ctx context.Context) ([]USBDevice, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	var count uint32
	structSize := uint32(unsafe.Sizeof(rawInputDeviceList{}))
	result, _, callErr := getRawInputDeviceList.Call(0, uintptr(unsafe.Pointer(&count)), uintptr(structSize))
	if uint32(result) == ^uint32(0) {
		return nil, fmt.Errorf("get Raw Input device count: %v", callErr)
	}
	if count == 0 {
		return nil, nil
	}
	items := make([]rawInputDeviceList, count)
	result, _, callErr = getRawInputDeviceList.Call(
		uintptr(unsafe.Pointer(&items[0])),
		uintptr(unsafe.Pointer(&count)),
		uintptr(structSize),
	)
	if uint32(result) == ^uint32(0) {
		return nil, fmt.Errorf("list Raw Input devices: %v", callErr)
	}
	if count > uint32(len(items)) {
		count = uint32(len(items))
	}
	var names []string
	for _, item := range items[:count] {
		var nameLength uint32
		result, _, _ = getRawInputDeviceInfoW.Call(
			item.Device,
			0x20000007,
			0,
			uintptr(unsafe.Pointer(&nameLength)),
		)
		if uint32(result) == ^uint32(0) || nameLength == 0 {
			continue
		}
		buffer := make([]uint16, nameLength+1)
		result, _, _ = getRawInputDeviceInfoW.Call(
			item.Device,
			0x20000007,
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(unsafe.Pointer(&nameLength)),
		)
		if uint32(result) == ^uint32(0) {
			continue
		}
		names = append(names, syscall.UTF16ToString(buffer))
	}
	return parseWindowsRawInputNames(names), nil
}

func splitUTF16MultiString(buffer []uint16) []string {
	var values []string
	start := 0
	for i, value := range buffer {
		if value != 0 {
			continue
		}
		if i == start {
			break
		}
		values = append(values, syscall.UTF16ToString(buffer[start:i]))
		start = i + 1
	}
	return values
}

func DefaultController() (config.Controller, error) {
	return config.Controller{Backend: "windows-ddc"}, nil
}

func ListMonitors(ctx context.Context, controller config.Controller) ([]Monitor, error) {
	if controller.Backend != "windows-ddc" {
		return nil, fmt.Errorf("unsupported Windows controller %q", controller.Backend)
	}
	physical, err := listPhysicalMonitors()
	if err != nil {
		return nil, err
	}
	defer destroyPhysicalMonitors(physical)
	if len(physical) == 0 {
		return nil, fmt.Errorf("Windows found no physical DDC/CI monitors")
	}

	script := `$names = @(Get-CimInstance -Namespace root\wmi -ClassName WmiMonitorID -ErrorAction SilentlyContinue |
    Where-Object { $_.Active } |
    ForEach-Object {
        -join ($_.UserFriendlyName | Where-Object { $_ -ne 0 } | ForEach-Object { [char]$_ })
    } |
    Where-Object { $_ })
$names | ConvertTo-Json -Compress`
	output, nameErr := runPowerShell(ctx, script)
	friendlyNames := parseJSONStringList(output)

	monitors := make([]Monitor, 0, len(physical))
	for index, item := range physical {
		name := syscall.UTF16ToString(item.Description[:])
		if nameErr == nil && index < len(friendlyNames) && strings.TrimSpace(friendlyNames[index]) != "" {
			name = friendlyNames[index]
		}
		monitors = append(monitors, Monitor{ID: strconv.Itoa(index), Name: name})
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
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	physical, err := listPhysicalMonitors()
	if err != nil {
		return err
	}
	defer destroyPhysicalMonitors(physical)
	if index >= len(physical) {
		return fmt.Errorf("monitor index %d is no longer available", index)
	}
	result, _, callErr := setVCPFeature.Call(physical[index].Handle, 0x60, uintptr(input))
	if result == 0 {
		return fmt.Errorf("SetVCPFeature failed: %v; check that DDC/CI is enabled in the monitor menu", callErr)
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

func listPhysicalMonitors() ([]physicalMonitor, error) {
	var monitors []physicalMonitor
	var callbackErr error
	callback := syscall.NewCallback(func(hMonitor, _ uintptr, _ uintptr, _ uintptr) uintptr {
		var count uint32
		result, _, callErr := getPhysicalCount.Call(hMonitor, uintptr(unsafe.Pointer(&count)))
		if result == 0 {
			callbackErr = fmt.Errorf("get physical monitor count: %v", callErr)
			return 0
		}
		if count == 0 {
			return 1
		}
		items := make([]physicalMonitor, count)
		result, _, callErr = getPhysicalMonitors.Call(
			hMonitor,
			uintptr(count),
			uintptr(unsafe.Pointer(&items[0])),
		)
		if result == 0 {
			callbackErr = fmt.Errorf("get physical monitors: %v", callErr)
			return 0
		}
		monitors = append(monitors, items...)
		return 1
	})
	result, _, callErr := enumDisplayMonitors.Call(0, 0, callback, 0)
	if callbackErr != nil {
		destroyPhysicalMonitors(monitors)
		return nil, callbackErr
	}
	if result == 0 {
		return nil, fmt.Errorf("enumerate display monitors: %v", callErr)
	}
	return monitors, nil
}

func destroyPhysicalMonitors(monitors []physicalMonitor) {
	for _, monitor := range monitors {
		_, _, _ = destroyPhysical.Call(monitor.Handle)
	}
}

func parseJSONStringList(output string) []string {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}
	if strings.HasPrefix(output, "[") {
		var values []string
		if json.Unmarshal([]byte(output), &values) == nil {
			return values
		}
		return nil
	}
	var value string
	if json.Unmarshal([]byte(output), &value) == nil {
		return []string{value}
	}
	return nil
}
