package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ThomasMcFarlane/kvmvidswitcher/internal/app"
	"github.com/ThomasMcFarlane/kvmvidswitcher/internal/config"
	"github.com/ThomasMcFarlane/kvmvidswitcher/internal/platform"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, in io.Reader, out, errOut io.Writer) error {
	if len(args) == 0 {
		printUsage(out)
		return nil
	}

	switch args[0] {
	case "setup":
		fs := flag.NewFlagSet("setup", flag.ContinueOnError)
		fs.SetOutput(errOut)
		configPath := fs.String("config", "", "configuration file path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		path, err := resolveConfigPath(*configPath)
		if err != nil {
			return err
		}
		return app.RunWizard(context.Background(), in, out, path)

	case "run":
		fs := flag.NewFlagSet("run", flag.ContinueOnError)
		fs.SetOutput(errOut)
		configPath := fs.String("config", "", "configuration file path")
		once := fs.Bool("once", false, "check once and switch for the current KVM state")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		path, err := resolveConfigPath(*configPath)
		if err != nil {
			return err
		}
		cfg, err := config.Load(path)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		logger := log.New(out, "", log.LstdFlags)
		if *once {
			return app.RunOnce(ctx, cfg, logger)
		}
		return app.RunDaemon(ctx, cfg, logger)

	case "switch":
		fs := flag.NewFlagSet("switch", flag.ContinueOnError)
		fs.SetOutput(errOut)
		configPath := fs.String("config", "", "configuration file path")
		input := fs.Int("input", -1, "override configured input code")
		state := fs.String("state", "connected", "configured state: connected or disconnected")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		path, err := resolveConfigPath(*configPath)
		if err != nil {
			return err
		}
		cfg, err := config.Load(path)
		if err != nil {
			return err
		}
		var code uint16
		switch strings.ToLower(*state) {
		case "connected", "windows", "present":
			code = cfg.Monitor.ConnectedInputCode
		case "disconnected", "mac", "away", "absent":
			code = cfg.Monitor.DisconnectedInputCode
		default:
			return fmt.Errorf("invalid switch state %q; use connected or disconnected", *state)
		}
		if *input >= 0 {
			code = uint16(*input)
		}
		if code == 0 {
			return fmt.Errorf("no input is configured for state %q", *state)
		}
		if err := platform.SwitchInput(context.Background(), cfg.Controller, cfg.Monitor.ID, code); err != nil {
			return err
		}
		fmt.Fprintf(out, "Switched %s to input %d.\n", cfg.Monitor.Name, code)
		return nil

	case "devices":
		devices, err := platform.ListUSBDevicesDetailed(context.Background())
		if err != nil {
			return err
		}
		if len(devices) == 0 {
			fmt.Fprintln(out, "No connected USB devices found.")
			return nil
		}
		for i, device := range devices {
			fmt.Fprintf(out, "%2d. %s  VID:%04X PID:%04X", i+1, device.Name, device.VendorID, device.ProductID)
			if device.Serial != "" {
				fmt.Fprintf(out, "  serial:%s", device.Serial)
			}
			fmt.Fprintln(out)
		}
		return nil

	case "monitors":
		controller, err := platform.DefaultController()
		if err != nil {
			return err
		}
		monitors, err := platform.ListMonitors(context.Background(), controller)
		if err != nil {
			return err
		}
		for i, monitor := range monitors {
			fmt.Fprintf(out, "%2d. %s  id:%s\n", i+1, monitor.Name, monitor.ID)
		}
		return nil

	case "doctor":
		fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
		fs.SetOutput(errOut)
		configPath := fs.String("config", "", "configuration file path")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		path, err := resolveConfigPath(*configPath)
		if err != nil {
			return err
		}
		return app.Doctor(context.Background(), out, path)

	case "service":
		if len(args) < 2 {
			return errors.New("usage: kvmvidswitcher service <install|uninstall|status>")
		}
		fs := flag.NewFlagSet("service "+args[1], flag.ContinueOnError)
		fs.SetOutput(errOut)
		configPath := fs.String("config", "", "configuration file path")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		path, err := resolveConfigPath(*configPath)
		if err != nil {
			return err
		}
		switch args[1] {
		case "install":
			if _, err := config.Load(path); err != nil {
				return err
			}
			executable, err := os.Executable()
			if err != nil {
				return fmt.Errorf("find current executable: %w", err)
			}
			installed, err := platform.InstallService(executable, path)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "Background service installed using %s.\n", installed)
			return nil
		case "uninstall":
			if err := platform.UninstallService(); err != nil {
				return err
			}
			fmt.Fprintln(out, "Background service removed.")
			return nil
		case "status":
			status, err := platform.ServiceStatus()
			if err != nil {
				return err
			}
			fmt.Fprintln(out, status)
			return nil
		default:
			return fmt.Errorf("unknown service action %q", args[1])
		}

	case "version", "--version", "-v":
		fmt.Fprintln(out, version)
		return nil
	case "help", "--help", "-h":
		printUsage(out)
		return nil
	default:
		return fmt.Errorf("unknown command %q; run 'kvmvidswitcher help'", args[0])
	}
}

func resolveConfigPath(value string) (string, error) {
	if value != "" {
		absolute, err := filepath.Abs(value)
		if err != nil {
			return "", fmt.Errorf("resolve config path: %w", err)
		}
		return absolute, nil
	}
	return config.DefaultPath()
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, `KvmVidSwitcher keeps a monitor input in sync with a USB KVM.

Usage:
  kvmvidswitcher setup                 Guided configuration
  kvmvidswitcher run                   Run the monitor in the foreground
  kvmvidswitcher run --once            Switch once for the current KVM state
  kvmvidswitcher switch [--state STATE] Test a configured input
  kvmvidswitcher switch --input N       Force a specific input code
  kvmvidswitcher devices               List connected USB devices
  kvmvidswitcher monitors              List controllable monitors
  kvmvidswitcher doctor                Check the configuration and hardware
  kvmvidswitcher service install       Start automatically in the background
  kvmvidswitcher service status        Show background-service status
  kvmvidswitcher service uninstall     Remove the background service
  kvmvidswitcher version               Show the version

Add --config PATH to setup, run, switch, doctor, or service commands to use a
non-default configuration file.`)
}
