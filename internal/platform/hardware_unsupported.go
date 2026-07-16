//go:build !darwin && !windows

package platform

import (
	"context"
	"fmt"

	"github.com/ThomasMcFarlane/kvmvidswitcher/internal/config"
)

func ListUSBDevices(context.Context) ([]USBDevice, error) {
	return nil, fmt.Errorf("this operating system is not supported")
}

func DefaultController() (config.Controller, error) {
	return config.Controller{}, fmt.Errorf("this operating system is not supported")
}

func ListMonitors(context.Context, config.Controller) ([]Monitor, error) {
	return nil, fmt.Errorf("this operating system is not supported")
}

func SwitchInput(context.Context, config.Controller, string, uint16) error {
	return fmt.Errorf("this operating system is not supported")
}
