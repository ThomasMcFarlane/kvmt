package platform

import "testing"

func TestParseDarwinIOReg(t *testing.T) {
	t.Parallel()
	output := `+-o USB Receiver@00100000  <class IOUSBHostDevice>
  | {
  |   "USB Product Name" = "KVM Receiver"
  |   "USB Serial Number" = "ABC123"
  |   "idVendor" = 1133
  |   "idProduct" = 50475
  | }
  +-o USB Hub@00200000  <class IOUSBHostDevice>
    {
      "kUSBProductString" = "USB Hub"
      "idVendor" = 8457
      "idProduct" = 2082
    }`

	devices := parseDarwinIOReg(output)
	if len(devices) != 2 {
		t.Fatalf("got %d devices, want 2: %#v", len(devices), devices)
	}
	if got := devices[0]; got.Name != "KVM Receiver" || got.VendorID != 1133 || got.ProductID != 50475 || got.Serial != "ABC123" {
		t.Fatalf("unexpected first device: %#v", got)
	}
	if got := devices[1]; got.Name != "USB Hub" || got.VendorID != 8457 || got.ProductID != 2082 {
		t.Fatalf("unexpected second device: %#v", got)
	}
}

func TestParseM1DDCDisplays(t *testing.T) {
	t.Parallel()
	output := `[1] DELL U2723QE (320C4B79-F89E-4601-8592-DA40A547289B)
[2] LG HDR 4K (A115A115-18BB-4EF3-AA11-555555555555)`

	monitors, err := parseM1DDCDisplays(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(monitors) != 2 {
		t.Fatalf("got %d monitors, want 2", len(monitors))
	}
	if monitors[0].Name != "DELL U2723QE" || monitors[0].ID != "320C4B79-F89E-4601-8592-DA40A547289B" {
		t.Fatalf("unexpected monitor: %#v", monitors[0])
	}
}

func TestMatchesHonoursOptionalSerial(t *testing.T) {
	t.Parallel()
	device := USBDevice{Name: "Receiver", VendorID: 0x046d, ProductID: 0xc52b, Serial: "one"}
	if !Matches(device, deviceConfig(0x046d, 0xc52b, "")) {
		t.Fatal("device should match without a configured serial")
	}
	if !Matches(device, deviceConfig(0x046d, 0xc52b, "one")) {
		t.Fatal("device should match the same serial")
	}
	if Matches(device, deviceConfig(0x046d, 0xc52b, "two")) {
		t.Fatal("device should not match a different serial")
	}
}
