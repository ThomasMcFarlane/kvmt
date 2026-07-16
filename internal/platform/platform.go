package platform

import (
	"context"

	"github.com/ThomasMcFarlane/kvmvidswitcher/internal/config"
)

type USBDevice struct {
	Name      string
	VendorID  uint16
	ProductID uint16
	Serial    string
}

type Monitor struct {
	ID   string
	Name string
}

func Matches(device USBDevice, match config.DeviceConfig) bool {
	if device.VendorID != match.VendorID || device.ProductID != match.ProductID {
		return false
	}
	return match.Serial == "" || device.Serial == match.Serial
}

func DevicePresent(ctx context.Context, match config.DeviceConfig) (bool, error) {
	devices, err := ListUSBDevices(ctx)
	if err != nil {
		return false, err
	}
	for _, device := range devices {
		if Matches(device, match) {
			return true, nil
		}
	}
	return false, nil
}
