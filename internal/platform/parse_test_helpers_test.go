package platform

import "github.com/ThomasMcFarlane/kvmvidswitcher/internal/config"

func deviceConfig(vendor, product uint16, serial string) config.DeviceConfig {
	return config.DeviceConfig{VendorID: vendor, ProductID: product, Serial: serial}
}
