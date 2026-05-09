# Changelog

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
