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
	if !present {
		logger.Printf("KVM device %s is not connected; no switch needed", cfg.Device.Name)
		return nil
	}
	if err := platform.SwitchInput(ctx, cfg.Controller, cfg.Monitor.ID, cfg.Monitor.InputCode); err != nil {
		return err
	}
	logger.Printf("switched %s to input %d", cfg.Monitor.Name, cfg.Monitor.InputCode)
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
	retryAfter := time.Duration(cfg.RetrySeconds) * time.Second
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
	var switchPending bool
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
			switchPending = present && cfg.SwitchOnStart
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
				switchPending = true
				nextRetry = time.Time{}
			} else {
				logger.Printf("KVM switched away from this computer")
				switchPending = false
			}
		}

		if !stablePresent || !switchPending || (!nextRetry.IsZero() && now.Before(nextRetry)) {
			return nil
		}
		if err := switchInput(ctx, cfg.Controller, cfg.Monitor.ID, cfg.Monitor.InputCode); err != nil {
			nextRetry = now.Add(retryAfter)
			logger.Printf("monitor switch failed; retrying in %s: %v", retryAfter, err)
			return nil
		}
		switchPending = false
		nextRetry = time.Time{}
		logger.Printf("switched %s to input %d", cfg.Monitor.Name, cfg.Monitor.InputCode)
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
