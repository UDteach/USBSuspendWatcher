# USB Suspend Watch

USB Suspend Watch is an installer-free Windows desktop utility for watching connected USB devices and recording suspected USB Selective Suspend transitions.

The v0.8.5 release uses one production-ready monitoring layer and one lab-only experimental layer:

- Simple mode: runs without elevation, watches `WM_DEVICECHANGE`, polls SetupAPI, and reads `SPDRP_DEVICE_POWER_DATA`.
- Experimental ETW mode: starts from the GUI button and may show UAC because Windows requires elevated rights for USB ETW sessions.

No driver, service, installer, USBPcap dependency, or telemetry is used.

## Features

- Desktop GUI with a Japanese/English language dropdown.
- Shows a monitoring status summary with connected USB count, low-power device count, suspected suspend count, resume count, privilege, and log path.
- Lists currently connected USB devices as a tree-like list with parent/hub rows above their child device rows, including USB3 hub, xHCI, USB4 router, Thunderbolt, and USB-C UCSI topology hints when Windows exposes them.
- Defaults to an `FTDI COM only` target filter so FTDI-style USB serial adapters and their related converter nodes are easier to inspect. Switch to `All USB` to see every USB device.
- Groups FTDI adapter candidates by logical evidence so `USB Serial Port (COMxx)` and `USB Serial Converter` can be inspected together without assuming they are definitely the same physical device.
- Lets you enable or disable monitoring per connected USB device, parent hub, USB3 hub, USB4 router, or USB-C topology row with checkboxes, using specific identity keys before broader fallback evidence.
- Keeps the selected connected device as the current watch target across refreshes and reconnects when stable evidence such as VID/PID plus serial, logical group, related instance IDs, instance ID, or COM port allows it.
- Shows a USB changes / transitions pane below the connected-device list for D0/D3, PnP, suspend/resume, and system sleep/wake sequence tracking.
- Shows a selected-device sequence pane for D0/D3, PnP, parent/hub, wake, and related converter/port events observed after the app started.
- Separates the diagnostic summary from pretty-printed raw JSON evidence so the exact D0/D3, parent-state, wake, and same-device-candidate evidence can be copied.
- Opens a dedicated device details window when a connected USB device row is double-clicked.
- Records PnP arrival and removal events.
- Records system sleep and wake broadcasts so USB changes can be correlated with PC suspend/resume.
- Captures `powercfg /lastwake` after wake broadcasts when Windows allows it and labels wake confidence as high, medium, low, or unknown based on available evidence.
- Filters the visible event timeline by event type, confidence, and text search.
- Filters the visible event timeline by display level; the default hides noisy `info` events.
- Normalizes events into:
  - `power_d0_exit`
  - `power_d0_entry`
  - `idle_notification`
  - `pnp_arrival`
  - `pnp_removal`
  - `suspect_suspend`
  - `resume`
  - `system_sleep`
  - `system_wake`
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

The ETW helper is not considered production-ready in v0.8.5 because provider behavior differs by Windows build, permissions, and USB stack provider.

For lab testing, click `Start ETW (experimental)`. Depending on the machine policy, this may show UAC. If UAC appears, approve it to start the elevated helper process.
If no helper log appears within 45 seconds, the GUI records a retryable error so the app does not wait forever. The helper enables USB ETW providers one by one; if one provider is unavailable, the others can still run and the unavailable provider is written to the ETW helper log.

The helper subscribes to:

- `Microsoft-Windows-USB-USBHUB3`
- `Microsoft-Windows-USB-UCX`
- `Microsoft-Windows-USB-USBXHCI`

It attempts to focus on USB power-related events by enabling the `Power` provider keyword, including USBHUB3 D0 entry/exit and idle-notification events. The GUI hides and does not retain `info` events by default; choose `All` in the timeline level filter before starting ETW when you need raw ETW chatter for lab debugging.

For lab-only USB rundown capture, set this additional environment variable before starting ETW:

```powershell
$env:USB_SUSPEND_WATCH_ETW_RUNDOWN = "1"
```

For production-grade ETW validation today, use Microsoft `logman` traces and compare them with this app's simple-mode timeline.

When the GUI starts an elevated helper, the status area keeps showing the GUI's own privilege and adds the helper privilege once the helper writes its startup log. This makes it clear whether the GUI is still a standard-user process while the ETW helper is actually elevated.

## Device Evidence And UI Layout

The main window is split by role:

- Left top: currently connected USB devices.
- Left middle: FTDI adapter candidate groups.
- Left bottom: USB changes and transitions observed after this app session started.
- Right top: the full event timeline.
- Right middle: the selected device's session sequence.
- Right bottom: diagnostic summary and raw JSON evidence.

The connected-device list shows the parent/hub chain as tree-like rows rather than packing it into a single table cell. The selected-device diagnostic area and double-click details window show the same relationship as a hanging tree with line characters.

The selected-device diagnostic area includes the evidence used for simple-mode power classification:

- `SPDRP_DEVICE_POWER_DATA` raw bytes.
- `CM_POWER_DATA.PD_MostRecentPowerState`, mapped to D0/D1/D2/D3.
- Previous/current power transition evidence such as `D0 -> D3` or `D3 -> D0` in event details and selected-device sequences.
- Compact transition evidence in the visible event timeline, so rows show both what changed and the source evidence used for the judgment.
- COM port name from the device registry `PortName` value when available.
- Whether the device looks like the FTDI USB serial target under inspection.
- VID/PID, revision, serial, physical device object name, location paths, and parent/hub instance chain.
- Logical group, relation role, and related instance IDs for USB Serial Port / USB Serial Converter same-device candidates.
- Same-device candidate score and reasons: serial match is 90%, parent-instance match is 70%, location-path match is 60%, and VID/PID-only is 0% because it is not enough evidence.
- Parent/hub power states, including a `parent_low_power_child_d0` warning when a child reports D0 while a parent or hub reports D1/D2/D3.
- USB3/USB4/Thunderbolt/USB-C topology hints from parent services such as `USBHUB3`, `USBXHCI`, `Usb4HostRouter`, `Usb4DeviceRouter`, and `UcmUcsiCx`.
- A line-drawn parent/hub tree that shows parent hubs above the selected device and related converter/port candidates below it.
- Connected-at, last-changed, and recent per-device event sequence from the current app session.
- Wake correlation for nearby USB/PnP/D0/D3 events and `powercfg /lastwake` output after PC resume.

FTDI-style USB serial devices that expose both `USB Serial Port (COMxx)` and `USB Serial Converter` can share VID/PID. The app now labels them as the same physical-adapter candidate only when it can match VID/PID plus serial, parent instance, or location paths. VID/PID alone is not enough and is not treated as same-device evidence.

The app does not infer historical plug/unplug or D0/D3 transitions from before startup. It separates the current SetupAPI snapshot from events observed in the current session.

## Logs

Logs are stored next to the executable under `logs/` when writable.

If that location is not writable, logs fall back to:

```text
%LOCALAPPDATA%\UsbSuspendWatch\logs
```

Each log line is one JSON object.
Power transition and PnP events include a `raw` object with the SetupAPI evidence above. ETW events include provider properties in the same `raw` object. Wake events may include `lastwake`, `lastwake_error`, `wake_confidence`, `wake_reasons`, and `wake_correlation`.

New v0.8.2 raw keys are additive and keep existing JSONL compatibility. They may include `diagnostic_score`, `diagnostic_reasons`, `session_started_at`, and `diagnostic_summary`.

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
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...
.\build.ps1 -Version v0.8.5
```

`go test -race` requires CGO and a C compiler on Windows. The release package is built with `CGO_ENABLED=0`.

## References

- [USB Event Tracing for Windows](https://learn.microsoft.com/en-us/windows-hardware/drivers/usbcon/usb-event-tracing-for-windows)
- [How to capture a USB event trace with Logman](https://learn.microsoft.com/en-us/windows-hardware/drivers/usbcon/how-to-capture-a-usb-event-trace)
- [Microsoft USBView sample](https://learn.microsoft.com/en-us/samples/microsoft/windows-driver-samples/usbview-sample-application/)
