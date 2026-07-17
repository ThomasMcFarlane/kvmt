# KVMT (KVM Toggler)

KVMT keeps a monitor's video input synchronised with a USB KVM. It is designed
for a Mac connected through a USB-C dock that cannot pass DDC/CI: KVMT runs on
Windows and switches both monitor inputs without any connection between the
two computers.

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

- Windows 10 or 11.
- A monitor with DDC/CI enabled in its on-screen menu.
- A Windows video connection that passes DDC/CI.
- A KVM that moves a USB hub, keyboard, mouse, or receiver between computers.

The Mac dock does not need to pass DDC/CI because the Mac never sends a monitor
command.

## Install on Windows

1. Download `kvmt-windows-amd64.exe` from the latest GitHub release.
2. In PowerShell, run:

   ```powershell
   .\kvmt-windows-amd64.exe install
   ```

3. Open a new terminal. `kvmt` is now available on your user `PATH`.
4. Run `kvmt setup` and complete the guided configuration.
5. Run `kvmt service install` to start the invisible background service.

Administrator access is not required. The executable is installed to
`%LOCALAPPDATA%\Programs\kvmt\bin\kvmt.exe`; its bin directory is added to the
user `PATH`.

### Upgrade from KvmVidSwitcher

The first configuration-aware `kvmt` command automatically copies the existing
configuration from `%APPDATA%\KvmVidSwitcher\config.json` to
`%APPDATA%\kvmt\config.json`.
`kvmt service install` removes the old Scheduled Task and stops the old daemon.

## Configure

The Mac does not need KVMT installed.

1. Switch the KVM to Windows.
2. Run `kvmt setup`.
3. Select the Windows monitor input, then the Mac monitor input.
4. Use assisted USB detection. Switch away for about three seconds, then back
   during the ten-second observation window.
5. Select a USB device that follows the KVM.
6. Run `kvmt doctor`.
7. Run `kvmt service install`.

Assisted detection also records a disappearing keyboard or mouse as the fast
trigger. This avoids Windows' delayed PnP removal for generic USB hubs.

## Commands

```text
kvmt install
kvmt update --check
kvmt update
kvmt setup
kvmt edit
kvmt run
kvmt run --once
kvmt switch --state disconnected
kvmt switch --state connected
kvmt devices
kvmt monitors
kvmt doctor
kvmt debug [--lines 100]
kvmt service install
kvmt service status
kvmt service uninstall
kvmt version
```

Use `--config PATH` with configuration-aware commands to use a different JSON
file.

### Edit selected settings

Run `kvmt edit` to change only the KVM USB device, monitor, Windows input, Mac
input, or monitor tests. Untouched settings are preserved. Run
`kvmt service install` after saving to restart the daemon with the new config.

### Service status and diagnostics

`kvmt service status` prints the task state and the active configuration,
including the USB device, fast trigger, monitor, inputs, polling, debounce, and
retry settings.

After reproducing a slow switch, run `kvmt debug`. It prints the latest 40
timestamped service-log lines. Use `kvmt debug --lines 100` for more history.
The Windows log is stored at `%APPDATA%\kvmt\service.log`.

## Updates

`kvmt update --check` checks GitHub's latest published release without changing
anything. `kvmt update` downloads the matching binary, verifies GitHub's
SHA-256 asset digest, atomically replaces the installed executable, and
restarts an installed service.

Unauthenticated updates work once the repository is public. While it remains
private, set a read-only `GITHUB_TOKEN` in the environment before running the
update command.

## Input codes

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

- Explicit Raw Input triggers are polled every 100 milliseconds and switch
  after one matching state change.
- Generic USB devices remain available as a slower PnP fallback.
- Arrival selects the Windows input; departure selects the Mac input.
- Failed DDC/CI commands retry every 500 milliseconds.
- The Windows Scheduled Task launches through `wscript.exe` with window style
  `0`; no terminal window is created or minimised.
- Monitor commands and transition timestamps are written to the service log.

## Open-source safety

The repository contains no credentials, generated configuration, telemetry, or
user data. The complete Git history is scanned with Gitleaks before release.
See [SECURITY.md](SECURITY.md) for vulnerability reporting.

## Build

Go 1.24 or newer is required.

```sh
make test
make build
make release VERSION=v0.4.0
```

`make release` creates Apple Silicon macOS and x64 Windows binaries plus
SHA-256 checksums. The project intentionally has no CI configuration.
