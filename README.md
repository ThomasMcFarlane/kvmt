# KvmVidSwitcher

KvmVidSwitcher keeps a monitor's video input synchronised with a USB KVM.
For a Mac connected through a dock that blocks DDC/CI, run KvmVidSwitcher only
on Windows. Windows switches to its own monitor input when the KVM USB device
appears, and switches to the Mac input when that device disappears.

```text
MacBook via USB-C dock ── HDMI input ─┐
                                     ├── Monitor
Windows ───────────── DisplayPort ────┘
    │
    └──── USB KVM
          │
          ├── connected to Windows → select Windows input
          └── disconnected         → select Mac input
```

## Requirements

- A monitor with DDC/CI enabled in its on-screen menu.
- A KVM that moves at least one identifiable USB device between computers.
- Windows: Windows 10 or 11. Monitor control uses the native Windows DDC/CI API.

The Windows video connection must pass DDC/CI. The Mac dock does not need to
pass DDC/CI because it never sends a monitor command.

## Install

Download the Windows binary from the GitHub release. Rename
`kvmvidswitcher-windows-amd64.exe` to
`kvmvidswitcher.exe` and place it somewhere on your `PATH`, or run it using its
full path.

## Configure Windows

The Mac does not need KvmVidSwitcher installed.

1. Switch the KVM to Windows.
2. Run `kvmvidswitcher.exe setup`.
3. Select the Windows monitor input, then the Mac monitor input.
4. Use assisted USB detection. Switch the KVM away for about three seconds,
   then back during the ten-second observation window.
5. Select a USB device that follows the KVM, then accept the monitor test.
6. Run `kvmvidswitcher.exe doctor`.
7. Run `kvmvidswitcher.exe service install`.

The installer copies the binary to a stable per-user location and creates a
logon Scheduled Task. Administrator access is not required.

## Commands

```text
kvmvidswitcher setup
kvmvidswitcher run
kvmvidswitcher run --once
kvmvidswitcher switch
kvmvidswitcher switch --state disconnected
kvmvidswitcher switch --state connected
kvmvidswitcher devices
kvmvidswitcher monitors
kvmvidswitcher doctor
kvmvidswitcher debug
kvmvidswitcher debug --lines 100
kvmvidswitcher service install
kvmvidswitcher service status
kvmvidswitcher service uninstall
```

Use `--config PATH` with configuration-aware commands to use a different JSON
file. By default, configuration is stored in the operating system's per-user
configuration directory.

### Debug switching delays

After reproducing a slow switch, return to Windows and run:

```powershell
kvmvidswitcher.exe debug
```

This prints the latest 40 background-service log lines, including timestamps
for KVM detection, monitor commands, failures, and retries. Use `--lines 100`
to show more history. Copy the output when reporting a switching problem.

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

- USB presence uses native Windows device enumeration every 250 milliseconds.
- Two matching checks debounce a KVM transition, giving roughly 0.5-second
  detection without launching PowerShell in the monitoring loop.
- KVM arrival selects the configured Windows input.
- KVM departure selects the configured Mac input.
- A switch is issued only after a state transition, not on every poll.
- A failed DDC/CI command is retried every 500 milliseconds while the detected
  KVM state remains unchanged.
- Starting the background process selects the input matching the current KVM
  state once.
- The Scheduled Task launches through `wscript.exe`, a GUI host with no console
  window. The daemon itself is started with Windows window style `0`.
- Service installation uses a short launcher script to stay below Windows Task
  Scheduler's 261-character action limit.
- Keyboard and mouse receivers use Windows Raw Input presence, avoiding the
  several-second PnP removal delay when the KVM switches away.
- The background service writes timestamped diagnostics to
  `%LOCALAPPDATA%\KvmVidSwitcher\service.log`.

## Build

Go 1.24 or newer is required.

```sh
make test
make build
make release VERSION=v0.1.0
```

`make release` creates Apple Silicon macOS and x64 Windows binaries plus
SHA-256 checksums. The project intentionally has no CI configuration.
