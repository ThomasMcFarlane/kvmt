# KvmVidSwitcher

KvmVidSwitcher keeps a monitor's video input synchronised with a USB KVM.
Run it on both computers. When the KVM's USB devices move to a computer, that
computer tells the monitor to select its configured video input.

```text
MacBook ── video input A ─┐
                         ├── Monitor
Windows ── video input B ─┘
    │                │
    └──── USB KVM ───┘
          switch
             │
             └── detected locally → select that computer's monitor input
```

## Requirements

- A monitor with DDC/CI enabled in its on-screen menu.
- A KVM that moves at least one identifiable USB device between computers.
- macOS: Apple Silicon and [`m1ddc`](https://github.com/waydabber/m1ddc).
- Windows: Windows 10 or 11. Monitor control uses the native Windows DDC/CI API.

Some docks and video adaptors do not pass DDC/CI commands. Direct DisplayPort
or USB-C connections are generally the most reliable.

## Install

Download the binary for each computer from the GitHub release.

On macOS:

```sh
brew install m1ddc
chmod +x kvmvidswitcher-darwin-arm64
sudo mv kvmvidswitcher-darwin-arm64 /usr/local/bin/kvmvidswitcher
```

On Windows, rename `kvmvidswitcher-windows-amd64.exe` to
`kvmvidswitcher.exe` and place it somewhere on your `PATH`, or run it using its
full path.

## Configure both computers

Complete these steps once on the MacBook and once on Windows:

1. Switch the KVM to the computer being configured.
2. Run `kvmvidswitcher setup`.
3. Select the monitor and the video input connected to that computer.
4. Use assisted USB detection. Switch the KVM away for about three seconds,
   then back during the ten-second observation window.
5. Select a USB device that follows the KVM, then accept the monitor test.
6. Run `kvmvidswitcher doctor`.
7. Run `kvmvidswitcher service install`.

The installer copies the binary to a stable per-user location. It creates a
LaunchAgent on macOS and a logon Scheduled Task on Windows. Administrator
access is not required for the background service.

## Commands

```text
kvmvidswitcher setup
kvmvidswitcher run
kvmvidswitcher run --once
kvmvidswitcher switch
kvmvidswitcher devices
kvmvidswitcher monitors
kvmvidswitcher doctor
kvmvidswitcher service install
kvmvidswitcher service status
kvmvidswitcher service uninstall
```

Use `--config PATH` with configuration-aware commands to use a different JSON
file. By default, configuration is stored in the operating system's per-user
configuration directory.

## Input codes

The wizard provides the standard DDC/CI input-select values:

| Input | Decimal | Hex |
|---|---:|---:|
| DisplayPort 1 | 15 | `0x0f` |
| DisplayPort 2 | 16 | `0x10` |
| HDMI 1 | 17 | `0x11` |
| HDMI 2 | 18 | `0x12` |
| USB-C | 27 | `0x1b` |

Monitor manufacturers sometimes use non-standard values. Choose `Custom code`
in the wizard when needed.

## Behaviour

- USB presence is polled once per second.
- Two matching checks debounce a KVM transition.
- A switch is issued only when the KVM arrives, not on every poll.
- A failed DDC/CI command is retried every five seconds while the KVM remains
  connected.
- Starting the background process while the KVM is already connected selects
  the configured input once.

## Build

Go 1.24 or newer is required.

```sh
make test
make build
make release VERSION=v0.1.0
```

`make release` creates Apple Silicon macOS and x64 Windows binaries plus
SHA-256 checksums. The project intentionally has no CI configuration.
