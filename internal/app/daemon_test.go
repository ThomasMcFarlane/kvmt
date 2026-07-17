package app

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/ThomasMcFarlane/kvmt/internal/config"
)

func TestDaemonSwitchesOnConnectAfterDebounce(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.PollIntervalMS = 5
	cfg.DebounceChecks = 2
	cfg.SwitchOnStart = false

	var mu sync.Mutex
	checks := 0
	switches := 0
	check := func(context.Context, config.DeviceConfig) (bool, error) {
		mu.Lock()
		defer mu.Unlock()
		checks++
		return checks >= 3, nil
	}
	switchInput := func(context.Context, config.Controller, string, uint16) error {
		mu.Lock()
		defer mu.Unlock()
		switches++
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if err := runDaemon(ctx, cfg, log.New(io.Discard, "", 0), check, switchInput); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if switches != 1 {
		t.Fatalf("got %d switches, want 1", switches)
	}
}

func TestDaemonSwitchesOnStartWhenConnected(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.PollIntervalMS = 5
	cfg.SwitchOnStart = true

	switches := 0
	check := func(context.Context, config.DeviceConfig) (bool, error) { return true, nil }
	switchInput := func(context.Context, config.Controller, string, uint16) error {
		switches++
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := runDaemon(ctx, cfg, log.New(io.Discard, "", 0), check, switchInput); err != nil {
		t.Fatal(err)
	}
	if switches != 1 {
		t.Fatalf("got %d switches, want 1", switches)
	}
}

func TestDaemonSwitchesToAwayInputOnDisconnect(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.PollIntervalMS = 5
	cfg.DebounceChecks = 2
	cfg.SwitchOnStart = false

	var mu sync.Mutex
	checks := 0
	var switchedInputs []uint16
	check := func(context.Context, config.DeviceConfig) (bool, error) {
		mu.Lock()
		defer mu.Unlock()
		checks++
		return checks < 3, nil
	}
	switchInput := func(_ context.Context, _ config.Controller, _ string, input uint16) error {
		mu.Lock()
		defer mu.Unlock()
		switchedInputs = append(switchedInputs, input)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if err := runDaemon(ctx, cfg, log.New(io.Discard, "", 0), check, switchInput); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(switchedInputs) != 1 || switchedInputs[0] != 17 {
		t.Fatalf("got switched inputs %v, want [17]", switchedInputs)
	}
}

func TestDaemonRetriesFailedSwitchAtConfiguredInterval(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.PollIntervalMS = 5
	cfg.SwitchOnStart = true
	cfg.RetryIntervalMS = 15

	attempts := 0
	check := func(context.Context, config.DeviceConfig) (bool, error) { return true, nil }
	switchInput := func(context.Context, config.Controller, string, uint16) error {
		attempts++
		if attempts == 1 {
			return errors.New("temporary DDC failure")
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if err := runDaemon(ctx, cfg, log.New(io.Discard, "", 0), check, switchInput); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("got %d switch attempts, want 2", attempts)
	}
}

func TestDaemonUsesSingleCheckForFastTrigger(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.PollIntervalMS = 5
	cfg.DebounceChecks = 5
	cfg.SwitchOnStart = false
	cfg.Device.FastTrigger = &config.DeviceMatch{VendorID: 3, ProductID: 4}

	checks := 0
	switches := 0
	check := func(context.Context, config.DeviceConfig) (bool, error) {
		checks++
		return checks == 1, nil
	}
	switchInput := func(context.Context, config.Controller, string, uint16) error {
		switches++
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := runDaemon(ctx, cfg, log.New(io.Discard, "", 0), check, switchInput); err != nil {
		t.Fatal(err)
	}
	if switches != 1 {
		t.Fatalf("got %d switches, want 1", switches)
	}
}

func testConfig() config.Config {
	return config.Config{
		Device: config.DeviceConfig{Name: "KVM", VendorID: 1, ProductID: 2},
		Monitor: config.MonitorConfig{
			ID:                    "one",
			Name:                  "Monitor",
			ConnectedInputCode:    15,
			DisconnectedInputCode: 17,
		},
		Controller:      config.Controller{Backend: "test"},
		PollIntervalMS:  5,
		DebounceChecks:  2,
		RetrySeconds:    1,
		RetryIntervalMS: 20,
	}
}
