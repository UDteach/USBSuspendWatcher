# USB Suspend Watch

USB Suspend Watch is an installer-free Windows desktop utility for watching connected USB devices and recording suspected USB Selective Suspend transitions.

The v0.3.0 release uses one production-ready monitoring layer and one lab-only experimental layer:

- Simple mode: runs without elevation, watches `WM_DEVICECHANGE`, polls SetupAPI, and reads `SPDRP_DEVICE_POWER_DATA`.
- Experimental ETW mode: source code is included, but the release UI does not start it unless `USB_SUSPEND_WATCH_EXPERIMENTAL_ETW=1` is set.

No driver, service, installer, USBPcap dependency, or telemetry is used.

## Features

- Desktop GUI with a Japanese/English language dropdown.
- Shows a monitoring status summary with connected USB count, low-power device count, suspected suspend count, resume count, privilege, and log path.
- Lists currently connected USB devices.
- Records PnP arrival and removal events.
- Filters the visible event timeline by event type, confidence, and text search.
- Normalizes events into:
  - `power_d0_exit`
  - `power_d0_entry`
  - `idle_notification`
  - `pnp_arrival`
  - `pnp_removal`
  - `suspect_suspend`
  - `resume`
- Adds `source` and `confidence` to each event.
- Saves local JSON Lines logs.
- Exports the filtered visible event timeline.
- Minimizes to the system tray.
- Shows tray notifications only for suspected suspend, resume, and error events while minimized.
- Builds as a standalone Windows x64 `.exe`.

## Monitoring Modes

### Simple Mode

Simple mode starts automatically and does not require UAC.

It polls SetupAPI and treats this power-state pattern as a suspected suspend:

- `D0 -> D1/D2/D3`: `power_d0_exit` and `suspect_suspend`
- `D1/D2/D3 -> D0`: `power_d0_entry` and `resume`

This is an inference from Windows device power data, not a kernel trace.

### Experimental ETW Mode

The ETW helper is not considered production-ready in v0.3.0. It is disabled in the release UI by default because provider-enable behavior differs by Windows build, permissions, and USB stack provider.

For lab testing only, set this environment variable before starting the app:

```powershell
$env:USB_SUSPEND_WATCH_EXPERIMENTAL_ETW = "1"
.\dist\usb-suspend-watch.exe
```

Then click `Start ETW (experimental)`. Depending on the machine policy, this may show UAC.

The helper subscribes to:

- `Microsoft-Windows-USB-USBHUB3`
- `Microsoft-Windows-USB-UCX`
- `Microsoft-Windows-USB-USBXHCI`

It attempts to focus on USB power-related events, including USBHUB3 D0 entry/exit and idle-notification events.

For production-grade ETW validation today, use Microsoft `logman` traces and compare them with this app's simple-mode timeline.

## Logs

Logs are stored next to the executable under `logs/` when writable.

If that location is not writable, logs fall back to:

```text
%LOCALAPPDATA%\UsbSuspendWatch\logs
```

Each log line is one JSON object.

## Build

Requirements:

- Windows x64
- Go 1.25 or later

Build:

```powershell
.\build.ps1 -Version local
```

Outputs:

- `dist/usb-suspend-watch.exe`
- `dist/usb-suspend-watch-x64.zip`

The build embeds a Common Controls v6 application manifest into the `.exe`, so the GUI does not require a sidecar manifest file.

## QA

Recommended checks before publishing:

```powershell
go mod verify
go test ./...
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
.\build.ps1 -Version v0.3.0
```

`go test -race` requires CGO and a C compiler on Windows. The release package is built with `CGO_ENABLED=0`.

## References

- [USB Event Tracing for Windows](https://learn.microsoft.com/en-us/windows-hardware/drivers/usbcon/usb-event-tracing-for-windows)
- [How to capture a USB event trace with Logman](https://learn.microsoft.com/en-us/windows-hardware/drivers/usbcon/how-to-capture-a-usb-event-trace)
- [Microsoft USBView sample](https://learn.microsoft.com/en-us/samples/microsoft/windows-driver-samples/usbview-sample-application/)
