# Changelog

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
