# Changelog

## v0.8.7

- Expanded SetupAPI collection to keep an all-present-devices index for parent resolution while still showing USB-related chains, allowing ACPI / PCI / xHCI / USB4 parent nodes to resolve instead of stopping at a USB-only snapshot.
- Added device identity fields to the connected USB tree, details, raw JSON, and snapshot logs: instance ID, parent instance ID, container ID, class GUID, driver, bus-reported description, PDO name, VID, PID, and location path.
- Decodes `SPDRP_DEVICE_POWER_DATA` as `CM_POWER_DATA`, including `PD_MostRecentPowerState`, capabilities, D1/D2/D3 latencies, power-state mapping, deepest system wake, and the explicit note that `PD_MostRecentPowerState` cannot distinguish D3hot from D3cold.
- Adds Config Manager status collection via `CM_Get_DevNode_Status`, including DN flags, problem code, and ConfigManagerErrorCode, with events for problem-code and status changes.
- Adds session Correlation IDs and per-session file names for device snapshots, events, ETW `.etl`, USBPcap `.pcapng`, and network-statistics logs.
- Writes periodic device tree snapshots every 5 seconds, switching to 1-second detailed logging for 60 seconds after D-state changes, parent mismatch, ProblemCode/status changes, missing devices, re-enumeration candidates, or errors.
- Emits new timeline events for D-state transitions, parent D-state mismatch, problem code, status change, missing device, re-enumeration candidate, last-seen stale, network statistics, and detailed logging start.
- Extends the ETW helper with optional `logman` `.etl` capture for USBHUB3, UCX, USBXHCI, Kernel-PnP, and Kernel-Power alongside the existing real-time JSONL tail.
- Adds USBPcap metadata hints for root hub, interface, bus/device address, endpoint fields, and unknown endpoint confidence when Windows/USBPcap cannot prove endpoint information.
- Captures PowerShell `Get-NetAdapter` / `Get-NetAdapterStatistics` data for USB network-class chains and logs it with the same timestamp/correlation ID.
- Adds diagnostic-cause candidates for USB power-management, NDIS/driver, USB transfer stall, and dongle FW/PHY hypotheses without treating any one signal as a definitive cause.

## v0.8.6

- Added optional USBPcap capture controls for the selected connected USB device, parent hub, or watched target.
- Detects `USBPcapCMD.exe` from `USBPCAP_CMD`, `PATH`, and common USBPcap/Wireshark extcap install locations without bundling or installing a driver.
- Reads USBPcap extcap interface/config output to match the selected target by COM port, serial, VID/PID, or device name, then starts `--devices <address>` capture when a device address can be matched.
- Falls back to capture-all with an explicit warning only when USBPcap exposes a single interface; with multiple unmatched interfaces it refuses to guess the Root Hub.
- Saves `.pcap` output and companion metadata JSON under the log folder, and adds searchable `usbpcap_*` raw evidence to JSONL events.
- Keeps USBPcap start/stop events visible even under the default timeline level and FTDI target filters.
- Added parser/planner tests for USBPcap extcap output and regression tests for USBPcap timeline visibility.

## v0.8.5

- Fixed parent/hub rows in the connected USB tree so root hubs, USB3 hubs, USB4 routers, and other parent nodes can be checked, watched, selected, and opened with double-click details.
- Added explicit previous/current power transition evidence such as `D0 -> D3` or `D3 -> D0` to event details and selected-device sequences.
- Added compact transition evidence to event-list messages so the visible timeline shows what changed and which power-state evidence produced the judgment.
- Added regression tests for watchable parent rows and visible transition evidence.

## v0.8.4

- Kept the selected connected USB row as a watch target across refreshes and reconnects when stable identity evidence can match the device again.
- Converted the connected USB list from a flat device table with a parent-tree cell into tree-like parent/hub rows with child device rows.
- Added USB3/USB4/Thunderbolt/USB-C topology hints so xHCI, USBHUB3, USB4 router, and UCSI parent nodes are visible when they are part of the device chain.
- Reduced GUI table churn by avoiding full connected-device resets when visible row data is unchanged and by inserting visible timeline rows instead of resetting the whole event table for every appended event.
- Tightened per-device monitoring keys so VID/PID-only or hardware-ID fallback evidence does not disable a different device when a specific identity such as VID/PID plus serial, logical group, instance ID, related ID, or COM port is available.
- Added regression tests for reconnect tracking, reused COM ports with different serials, broad hardware-ID fallback behavior, stale snapshot avoidance, and stable serial-qualified monitoring.

## v0.8.3

- Added a default `Parent tree` column to the connected USB table so parent/hub hanging relationships are visible without opening details.
- Added a double-click device details window for connected USB devices, with selected-device history, diagnostic summary, and raw JSON evidence.
- Changed the parent/hub detail rendering to use line-drawn hanging tree connectors for parent hubs, selected device, and related candidates.
- Added tests for the default parent-tree column and line-drawn parent-chain formatting.

## v0.8.2

- Added diagnostic scores and reasons for USB Serial Port / USB Serial Converter same-device candidates: serial match is 90%, parent-instance match is 70%, location-path match is 60%, and VID/PID-only remains 0%.
- Split the main UI into current connected devices, FTDI adapter groups, USB changes, full timeline, selected-device sequence, diagnostic summary, and pretty raw JSON evidence.
- Added `Parent D3` warning marks when a child reports D0 while a parent/hub reports D1/D2/D3.
- Added wake confidence and reasons to wake raw data using `powercfg /lastwake` and nearby USB/PnP/D0/D3 events.
- Added session-start metadata and clearer wording that historical transitions are only those observed after the app starts.
- Expanded raw JSON with additive keys such as `diagnostic_score`, `diagnostic_reasons`, `group_display_name`, `session_observed`, `session_started_at`, and `diagnostic_summary`.
- Added tests for diagnostic scoring, FTDI adapter grouping, wake confidence, pretty raw JSON, parent warning marks, session raw metadata, and selected-device sequence data.

## v0.8.1

- Added an `All USB` / `FTDI COM only` target filter. The default focuses the device list and event timeline on FTDI-style COM-port devices and their related converter nodes.
- Added logical grouping for USB Serial Port and USB Serial Converter nodes using VID/PID plus serial, parent instance, or location paths. VID/PID alone is intentionally not treated as the same physical device.
- Added parent/hub power-state comparison and a `parent_low_power_child_d0` diagnostic when a child reports D0 while a parent/hub node is in D1/D2/D3.
- Captured `powercfg /lastwake` on system wake broadcasts and added wake correlation details for nearby USB/PnP/D0/D3 events.
- Added a USB changes / transitions pane below the connected-device list for D0/D3, PnP, suspend/resume, and sleep/wake sequence tracking.
- Added an indented relation/hub tree in the details pane so parent hubs and related converter/port nodes are easier to follow.
- Expanded details and JSONL raw evidence with logical group, relation role, related instance IDs, parent states, and wake-source/correlation data.

## v0.8.0

- Show ETW helper privilege separately from the GUI privilege so elevated helper startup is visible even when the GUI remains a standard-user process.
- Added FTDI/USB serial diagnostics: COM port, serial/revision, physical device object, location paths, and parent/hub instance chain.
- Added connected-at and last-changed timestamps plus a selected-device recent sequence view.
- Added raw SetupAPI power evidence to device details and JSONL events, including `SPDRP_DEVICE_POWER_DATA` bytes and the `PD_MostRecentPowerState` value used for D0/D1/D2/D3.
- Added `WM_POWERBROADCAST` sleep/wake events to the timeline so system suspend/resume can be correlated with USB plug, removal, and power-state transitions.

## v0.7.2

- Started the ETW session before enabling USB providers and enabled providers one by one, so one unavailable provider no longer prevents all ETW monitoring.
- Added a GUI-side ETW startup timeout that records a clear error and allows retry if UAC is cancelled, hidden, or blocked before the helper writes its first log.
- Included enabled ETW provider names in the helper running event for easier field diagnostics.

## v0.7.1

- Added parent-process monitoring to the elevated ETW helper so an orphaned helper shuts down if the GUI exits unexpectedly.
- Reset the GUI ETW state when helper startup fails, making retry possible without restarting the app.
- Increased JSONL tailing tolerance for large ETW payloads.
- Hardened the release workflow with vet, module verification, staticcheck, govulncheck, and Node 24-compatible artifact upload.

## v0.7.0

- Made the `Start ETW experimental` button start the elevated ETW helper directly without requiring `USB_SUSPEND_WATCH_EXPERIMENTAL_ETW=1`.
- Kept ETW labeled experimental and UAC-gated, but removed the hidden release UI gate that made the button look inert.
- Updated documentation for the new ETW start flow.

## v0.6.0

- Fixed elevated ETW helper startup for USB providers on Windows by using plain provider enable parameters.
- Reduced ETW capture noise by limiting USB providers to the `Power` keyword by default.
- Made USB rundown capture opt-in with `USB_SUSPEND_WATCH_ETW_RUNDOWN=1`.
- Added a timeline display-level selector with `No info` as the default, plus `Important only` and `All`; default mode does not retain ETW `info` events in memory.
- Added tests for ETW provider parameters and display-level filtering.

## v0.5.0

- Added a current-state column to the connected USB device list.
- The state column shows monitoring-off targets, active D0 devices, low-power/suspected-suspend devices, removed devices, and unknown state in the selected language.
- Added the same current-state summary to the selected device details pane.
- Added unit tests for device state labels and localized device-table columns.

## v0.4.1

- Kept the language selector label as `Language` even in Japanese mode so English-speaking users can find it quickly.

## v0.4.0

- Added per-device monitoring checkboxes to the connected USB device list.
- New devices are monitored by default; unchecked devices remain disabled across refreshes within the running session.
- Suppressed device-specific timeline events, logs, and tray notifications for unchecked monitoring targets.
- Added monitored-target counts to the status summary.
- Added unit tests for monitoring state persistence and event suppression.

## v0.3.0

- Added a Japanese/English language dropdown in the main toolbar.
- Replaced always-bilingual GUI labels with single-language labels that update across buttons, filters, table headers, details, dialogs, and tray menu items.
- Kept event filters and current selection stable when switching languages.
- Added localization tests for event markers and core UI labels.

## v0.2.0

- Embedded a Common Controls v6 manifest so the Walk GUI starts as a standalone `.exe`.
- Improved the GUI with bilingual labels, monitoring summary, event filters, visible-log export, event markers, and quieter tray notifications.
- Split the Windows GUI implementation into focused table-model, filter, detail-formatting, and icon modules.
- Added tests for event filtering and timeline marker normalization.
- Primed the USB poller before background monitoring so the connected-device list is available immediately after startup.

## v0.1.0

- Initial MVP release.
- Added simple SetupAPI/WM_DEVICECHANGE monitoring.
- Included lab-only experimental ETW helper code for USBHUB3, UCX, and USBXHCI power events; disabled in the release UI by default.
- Added JSONL logging, timeline export, and tray minimization.
- Added unit tests for event normalization and Windows power-data parsing.
