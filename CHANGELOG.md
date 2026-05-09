# Changelog

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
