package platform

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	ioRegDeviceStart = regexp.MustCompile(`^[\s|]*\+-o (.+?)@`)
	ioRegNumber      = regexp.MustCompile(`"(idVendor|idProduct)" = (\d+)`)
	ioRegString      = regexp.MustCompile(`"(USB Product Name|kUSBProductString|USB Serial Number|kUSBSerialNumberString)" = "([^"]*)"`)
	m1ddcDisplay     = regexp.MustCompile(`^\s*\[\d+\]\s+(.+?)\s+\(([0-9A-Fa-f-]{8,})\)\s*$`)
)

func parseDarwinIOReg(output string) []USBDevice {
	var devices []USBDevice
	var current *USBDevice
	flush := func() {
		if current != nil && current.VendorID != 0 && current.ProductID != 0 {
			devices = append(devices, *current)
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if match := ioRegDeviceStart.FindStringSubmatch(line); match != nil {
			flush()
			current = &USBDevice{Name: strings.TrimSpace(match[1])}
			continue
		}
		if current == nil {
			continue
		}
		if match := ioRegNumber.FindStringSubmatch(line); match != nil {
			value, _ := strconv.ParseUint(match[2], 10, 16)
			if match[1] == "idVendor" {
				current.VendorID = uint16(value)
			} else {
				current.ProductID = uint16(value)
			}
			continue
		}
		if match := ioRegString.FindStringSubmatch(line); match != nil {
			switch match[1] {
			case "USB Product Name", "kUSBProductString":
				if match[2] != "" {
					current.Name = match[2]
				}
			case "USB Serial Number", "kUSBSerialNumberString":
				current.Serial = match[2]
			}
		}
	}
	flush()
	return devices
}

func parseM1DDCDisplays(output string) ([]Monitor, error) {
	var monitors []Monitor
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		if match := m1ddcDisplay.FindStringSubmatch(scanner.Text()); match != nil {
			monitors = append(monitors, Monitor{ID: match[2], Name: strings.TrimSpace(match[1])})
		}
	}
	if len(monitors) == 0 {
		return nil, fmt.Errorf("m1ddc returned no controllable displays; output was %q", strings.TrimSpace(output))
	}
	return monitors, nil
}
