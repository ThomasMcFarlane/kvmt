package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ThomasMcFarlane/kvmvidswitcher/internal/config"
	"github.com/ThomasMcFarlane/kvmvidswitcher/internal/platform"
)

type deviceChecker func(context.Context, config.DeviceConfig) (bool, error)
type inputSwitcher func(context.Context, config.Controller, string, uint16) error

func RunDaemon(ctx context.Context, cfg config.Config, logger *log.Logger) error {
	return runDaemon(ctx, cfg, logger, platform.DevicePresent, platform.SwitchInput)
}

func RunOnce(ctx context.Context, cfg config.Config, logger *log.Logger) error {
	present, err := platform.DevicePresent(ctx, cfg.Device)
	if err != nil {
		return fmt.Errorf("detect KVM device: %w", err)
	}
	inputCode := cfg.InputCodeForDeviceState(present)
	if inputCode == 0 {
		logger.Printf("no monitor input is configured for KVM device state connected=%t", present)
		return nil
	}
	if err := platform.SwitchInput(ctx, cfg.Controller, cfg.Monitor.ID, inputCode); err != nil {
		return err
	}
	logger.Printf("switched %s to input %d", cfg.Monitor.Name, inputCode)
	return nil
}

func runDaemon(
	ctx context.Context,
	cfg config.Config,
	logger *log.Logger,
	check deviceChecker,
	switchInput inputSwitcher,
) error {
	interval := time.Duration(cfg.PollIntervalMS) * time.Millisecond
	retryAfter := time.Duration(cfg.RetryIntervalMS) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Printf(
		"watching for %s (VID:%04X PID:%04X) every %s",
		cfg.Device.Name,
		cfg.Device.VendorID,
		cfg.Device.ProductID,
		interval,
	)

	var stablePresent bool
	var initialised bool
	var candidate bool
	var candidateCount int
	var pendingInput uint16
	var nextRetry time.Time

	checkNow := func(now time.Time) error {
		present, err := check(ctx, cfg.Device)
		if err != nil {
			return fmt.Errorf("detect KVM device: %w", err)
		}

		if !initialised {
			initialised = true
			stablePresent = present
			candidate = present
			candidateCount = cfg.DebounceChecks
			if cfg.SwitchOnStart {
				pendingInput = cfg.InputCodeForDeviceState(present)
			}
			if present {
				logger.Printf("KVM device is connected")
			} else {
				logger.Printf("KVM device is not connected")
			}
		} else if present == candidate {
			candidateCount++
		} else {
			candidate = present
			candidateCount = 1
		}

		if candidateCount >= cfg.DebounceChecks && candidate != stablePresent {
			stablePresent = candidate
			if stablePresent {
				logger.Printf("KVM switched to this computer")
			} else {
				logger.Printf("KVM switched away from this computer")
			}
			pendingInput = cfg.InputCodeForDeviceState(stablePresent)
			nextRetry = time.Time{}
		}

		if pendingInput == 0 || (!nextRetry.IsZero() && now.Before(nextRetry)) {
			return nil
		}
		if err := switchInput(ctx, cfg.Controller, cfg.Monitor.ID, pendingInput); err != nil {
			nextRetry = now.Add(retryAfter)
			logger.Printf("monitor switch failed; retrying in %s: %v", retryAfter, err)
			return nil
		}
		switchedInput := pendingInput
		pendingInput = 0
		nextRetry = time.Time{}
		logger.Printf("switched %s to input %d", cfg.Monitor.Name, switchedInput)
		return nil
	}

	if err := checkNow(time.Now()); err != nil {
		logger.Printf("%v", err)
	}
	for {
		select {
		case <-ctx.Done():
			logger.Printf("stopped")
			return nil
		case now := <-ticker.C:
			if err := checkNow(now); err != nil {
				logger.Printf("%v", err)
			}
		}
	}
}
