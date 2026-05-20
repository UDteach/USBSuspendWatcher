package ui

import (
	"testing"

	"usb-suspend-watch/internal/model"
)

func TestDeviceMonitoringDefaultsOnAndPersistsByInstanceID(t *testing.T) {
	m := newDeviceTableModel()
	reader := model.DeviceSnapshot{
		InstanceID:   `USB\VID_0BDA&PID_0129\A`,
		HardwareID:   `USB\VID_0BDA&PID_0129`,
		FriendlyName: "USB Reader",
	}
	keyboard := model.DeviceSnapshot{
		InstanceID:   `USB\VID_1234&PID_5678\B`,
		HardwareID:   `USB\VID_1234&PID_5678`,
		FriendlyName: "USB Keyboard",
	}

	m.Set([]model.DeviceSnapshot{reader, keyboard})
	if !m.Checked(0) || !m.Checked(1) {
		t.Fatalf("new devices should be monitored by default")
	}

	if err := m.SetChecked(0, false); err != nil {
		t.Fatalf("SetChecked returned error: %v", err)
	}
	if m.Checked(0) {
		t.Fatalf("unchecked device should be unmonitored")
	}
	if !m.Checked(1) {
		t.Fatalf("other devices should stay monitored")
	}

	m.Set([]model.DeviceSnapshot{keyboard, reader})
	if m.Checked(1) {
		t.Fatalf("unchecked state should follow the same instance ID after refresh")
	}
	if got := m.MonitoredCount(); got != 1 {
		t.Fatalf("MonitoredCount = %d, want 1", got)
	}
}

func TestDeviceMonitoringDoesNotApplyBroadHardwareIDWhenSpecificIDExists(t *testing.T) {
	m := newDeviceTableModel()
	device := model.DeviceSnapshot{
		InstanceID: `USB\VID_0BDA&PID_0129\A`,
		HardwareID: `USB\VID_0BDA&PID_0129`,
		VID:        "0BDA",
		PID:        "0129",
	}

	m.Set([]model.DeviceSnapshot{device})
	if err := m.SetChecked(0, false); err != nil {
		t.Fatalf("SetChecked returned error: %v", err)
	}

	hardwareOnlyEventDevice := model.DeviceSnapshot{HardwareID: `USB\VID_0BDA&PID_0129`}
	if !m.IsMonitored(hardwareOnlyEventDevice) {
		t.Fatalf("broad hardware-only event should not inherit a specific instance monitoring state")
	}
}

func TestDeviceMonitoringCanUseBroadHardwareIDWhenNoSpecificIDExists(t *testing.T) {
	m := newDeviceTableModel()
	device := model.DeviceSnapshot{
		HardwareID:   `USB\VID_0BDA&PID_0129`,
		FriendlyName: "USB Reader",
	}

	m.Set([]model.DeviceSnapshot{device})
	if err := m.SetChecked(0, false); err != nil {
		t.Fatalf("SetChecked returned error: %v", err)
	}

	hardwareOnlyEventDevice := model.DeviceSnapshot{HardwareID: `USB\VID_0BDA&PID_0129`}
	if m.IsMonitored(hardwareOnlyEventDevice) {
		t.Fatalf("hardware-id fallback should be used when no specific identity exists")
	}
}

func TestDeviceMonitoringFollowsSerialAcrossReconnect(t *testing.T) {
	m := newDeviceTableModel()
	original := model.DeviceSnapshot{
		InstanceID:   `FTDIPORT\VID_0403+PID_6001+FT123\0000`,
		FriendlyName: "USB Serial Port",
		VID:          "0403",
		PID:          "6001",
		Serial:       "FT123",
		COMPort:      "COM52",
	}
	reconnected := model.DeviceSnapshot{
		InstanceID:   `FTDIPORT\VID_0403+PID_6001+FT123\0001`,
		FriendlyName: "USB Serial Port",
		VID:          "0403",
		PID:          "6001",
		Serial:       "FT123",
		COMPort:      "COM52",
	}

	m.Set([]model.DeviceSnapshot{original})
	if err := m.SetChecked(0, false); err != nil {
		t.Fatalf("SetChecked returned error: %v", err)
	}
	m.Set([]model.DeviceSnapshot{reconnected})

	if m.IsMonitored(reconnected) {
		t.Fatalf("serial-qualified monitoring choice should follow reconnect")
	}
}

func TestFindDeviceIndexFollowsSerialAcrossReconnect(t *testing.T) {
	target := model.DeviceSnapshot{VID: "0403", PID: "6001", Serial: "FT123", COMPort: "COM52"}
	devices := []model.DeviceSnapshot{
		{VID: "0403", PID: "6001", Serial: "FT999", COMPort: "COM53"},
		{VID: "0403", PID: "6001", Serial: "FT123", COMPort: "COM54"},
	}
	if got := findDeviceIndex(devices, target); got != 1 {
		t.Fatalf("findDeviceIndex = %d, want 1", got)
	}
}

func TestFindDeviceIndexDoesNotFollowReusedCOMWhenSerialDiffers(t *testing.T) {
	target := model.DeviceSnapshot{VID: "0403", PID: "6001", Serial: "FT123", COMPort: "COM52"}
	devices := []model.DeviceSnapshot{
		{VID: "0403", PID: "6001", Serial: "FT999", COMPort: "COM52"},
	}
	if got := findDeviceIndex(devices, target); got != -1 {
		t.Fatalf("findDeviceIndex = %d, want -1 for reused COM with different serial", got)
	}
}

func TestWatchDetailsRefreshesForWatchedDeviceAndSystemPower(t *testing.T) {
	watched := model.DeviceSnapshot{VID: "0403", PID: "6001", Serial: "FT123", COMPort: "COM52"}
	a := &app{watchedDevice: watched, hasWatchedDevice: true, watchFocusActive: true}

	if !a.shouldRefreshWatchDetailsForEvent(model.Event{Device: model.DeviceSnapshot{VID: "0403", PID: "6001", Serial: "FT123"}}) {
		t.Fatalf("watched device event should refresh watch details")
	}
	if !a.shouldRefreshWatchDetailsForEvent(model.Event{Type: model.EventSystemWake}) {
		t.Fatalf("system power event should refresh watch details for wake correlation")
	}
	if a.shouldRefreshWatchDetailsForEvent(model.Event{Device: model.DeviceSnapshot{VID: "0403", PID: "6001", Serial: "FT999"}}) {
		t.Fatalf("different serial device event should not refresh watch details")
	}
}

func TestDeviceMonitoringDoesNotUseBareSerialAcrossDifferentVIDPID(t *testing.T) {
	m := newDeviceTableModel()
	original := model.DeviceSnapshot{
		VID:     "0403",
		PID:     "6001",
		Serial:  "FT123",
		COMPort: "COM52",
	}
	other := model.DeviceSnapshot{
		VID:     "1234",
		PID:     "6001",
		Serial:  "FT123",
		COMPort: "COM53",
	}

	m.Set([]model.DeviceSnapshot{original})
	if err := m.SetChecked(0, false); err != nil {
		t.Fatalf("SetChecked returned error: %v", err)
	}
	m.Set([]model.DeviceSnapshot{other})

	if !m.IsMonitored(other) {
		t.Fatalf("bare serial should not disable a different VID/PID device")
	}
}

func TestDeviceMonitoringDoesNotUseVIDPIDFallbackWhenSerialDiffers(t *testing.T) {
	m := newDeviceTableModel()
	original := model.DeviceSnapshot{
		VID:     "0403",
		PID:     "6001",
		Serial:  "FT123",
		COMPort: "COM52",
	}
	other := model.DeviceSnapshot{
		VID:     "0403",
		PID:     "6001",
		Serial:  "FT999",
		COMPort: "COM53",
	}

	m.Set([]model.DeviceSnapshot{original})
	if err := m.SetChecked(0, false); err != nil {
		t.Fatalf("SetChecked returned error: %v", err)
	}
	m.Set([]model.DeviceSnapshot{other})

	if !m.IsMonitored(other) {
		t.Fatalf("VID/PID-only fallback should not disable a different serial device")
	}
}

func TestDeviceTableSetKeepsLatestSnapshotWhenVisibleRowUnchanged(t *testing.T) {
	m := newDeviceTableModel()
	first := model.DeviceSnapshot{
		InstanceID:   `USB\VID_0403&PID_6001\A`,
		FriendlyName: "USB Serial Port",
		VID:          "0403",
		PID:          "6001",
		Serial:       "FT123",
		Present:      true,
	}
	second := first
	second.Serial = "FT456"

	m.Set([]model.DeviceSnapshot{first})
	m.Set([]model.DeviceSnapshot{second})

	got, ok := m.Item(0)
	if !ok {
		t.Fatalf("expected current item")
	}
	if got.Serial != "FT456" {
		t.Fatalf("latest snapshot serial = %q, want FT456", got.Serial)
	}
}

func TestDeviceCurrentStateReflectsMonitoringAndPower(t *testing.T) {
	cases := []struct {
		name      string
		device    model.DeviceSnapshot
		monitored bool
		language  displayLanguage
		want      string
	}{
		{
			name:      "monitoring off",
			device:    model.DeviceSnapshot{Present: true, PowerState: model.PowerD0},
			monitored: false,
			language:  languageEnglish,
			want:      "Monitoring off",
		},
		{
			name:      "active japanese",
			device:    model.DeviceSnapshot{Present: true, PowerState: model.PowerD0},
			monitored: true,
			language:  languageJapanese,
			want:      "動作中 (D0)",
		},
		{
			name:      "low power english",
			device:    model.DeviceSnapshot{Present: true, PowerState: model.PowerD2},
			monitored: true,
			language:  languageEnglish,
			want:      "Low power / suspected suspend (D2)",
		},
		{
			name:      "unknown",
			device:    model.DeviceSnapshot{Present: true, PowerState: model.PowerUnknown},
			monitored: true,
			language:  languageEnglish,
			want:      "Unknown",
		},
		{
			name:      "removed",
			device:    model.DeviceSnapshot{InstanceID: `USB\VID_0BDA&PID_0129\A`, Present: false, PowerState: model.PowerD0},
			monitored: true,
			language:  languageEnglish,
			want:      "Removed",
		},
		{
			name:      "empty app-level device",
			device:    model.DeviceSnapshot{},
			monitored: true,
			language:  languageEnglish,
			want:      "Unknown",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deviceCurrentState(tc.device, tc.monitored, tc.language); got != tc.want {
				t.Fatalf("deviceCurrentState() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDeviceTableModelShowsStateColumns(t *testing.T) {
	m := newDeviceTableModel()
	device := model.DeviceSnapshot{
		InstanceID:   `USB\VID_0BDA&PID_0129\A`,
		FriendlyName: "USB Reader",
		Present:      true,
		PowerState:   model.PowerD3,
		COMPort:      "COM52",
		ParentStates: []model.ParentDeviceState{
			{DisplayName: "USB Hub", PowerState: model.PowerD0},
			{DisplayName: "USB xHCI Controller", PowerState: model.PowerD0},
		},
	}

	m.Set([]model.DeviceSnapshot{device})
	if got := m.RowCount(); got != 3 {
		t.Fatalf("tree row count = %d, want parent, parent, device", got)
	}
	if got := m.Value(0, 0); got != "USB xHCI Controller" {
		t.Fatalf("root parent row = %q", got)
	}
	if got := m.Value(1, 0); got != "└─ USB Hub" {
		t.Fatalf("nested parent row = %q", got)
	}
	if got := m.Value(2, 0); got != "   └─ USB Reader (COM52)" {
		t.Fatalf("device tree row = %q", got)
	}
	parent, ok := m.Item(0)
	if !ok || parent.InstanceID != "" && parent.DisplayName() == "" {
		t.Fatalf("parent row should be available as a watchable detail target: %#v", parent)
	}
	if got := m.Value(2, 1); got != "低電力 / Suspend疑い (D3)" {
		t.Fatalf("Japanese state column = %q", got)
	}

	m.SetLanguage(languageEnglish)
	if got := m.Value(2, 1); got != "Low power / suspected suspend (D3)" {
		t.Fatalf("English state column = %q", got)
	}

	if err := m.SetChecked(2, false); err != nil {
		t.Fatalf("SetChecked returned error: %v", err)
	}
	if got := m.Value(2, 1); got != "Monitoring off" {
		t.Fatalf("unchecked state column = %q", got)
	}
	if got := m.Value(2, 5); got != "COM52" {
		t.Fatalf("COM column = %q", got)
	}
}

func TestDeviceTableModelParentRowsAreWatchable(t *testing.T) {
	m := newDeviceTableModel()
	device := model.DeviceSnapshot{
		InstanceID:   `USB\VID_0BDA&PID_0129\A`,
		FriendlyName: "USB Reader",
		ParentStates: []model.ParentDeviceState{
			{InstanceID: `USB\ROOT_HUB30\1`, DisplayName: "USB Root Hub", Service: "USBHUB3", PowerState: model.PowerD0},
		},
	}

	m.Set([]model.DeviceSnapshot{device})
	parent, ok := m.Item(0)
	if !ok {
		t.Fatalf("parent row should return a synthetic device")
	}
	if parent.InstanceID != `USB\ROOT_HUB30\1` || parent.Service != "USBHUB3" {
		t.Fatalf("unexpected parent device: %#v", parent)
	}
	if !m.Checked(0) {
		t.Fatalf("parent row should be monitored by default")
	}
	if err := m.SetChecked(0, false); err != nil {
		t.Fatalf("SetChecked returned error: %v", err)
	}
	if m.IsMonitored(parent) {
		t.Fatalf("parent row monitoring state should be toggleable")
	}
}

func TestDeviceTableModelGroupsDevicesUnderSharedParentRows(t *testing.T) {
	m := newDeviceTableModel()
	parent := []model.ParentDeviceState{{DisplayName: "USB Hub", InstanceID: `USB\ROOT_HUB30\1`, PowerState: model.PowerD0}}
	m.Set([]model.DeviceSnapshot{
		{InstanceID: `USB\VID_0403&PID_6001\A`, FriendlyName: "USB Serial Port A", ParentStates: parent},
		{InstanceID: `USB\VID_0403&PID_6001\B`, FriendlyName: "USB Serial Port B", ParentStates: parent},
	})

	if got := m.RowCount(); got != 3 {
		t.Fatalf("tree row count = %d, want one shared parent plus two devices", got)
	}
	if got := m.Value(0, 0); got != "USB Hub" {
		t.Fatalf("shared parent row = %q", got)
	}
	if got := m.Value(1, 0); got != "└─ USB Serial Port A" {
		t.Fatalf("first child row = %q", got)
	}
	if got := m.Value(2, 0); got != "└─ USB Serial Port B" {
		t.Fatalf("second child row = %q", got)
	}
}

func TestDeviceHistoryMatchesCOMPortAndSerial(t *testing.T) {
	m := newEventTableModel(0)
	device := model.DeviceSnapshot{VID: "0403", PID: "6001", Serial: "FT123", COMPort: "COM52"}
	m.Add(model.Event{Device: model.DeviceSnapshot{COMPort: "COM52"}, Message: "arrival"})
	m.Add(model.Event{Device: model.DeviceSnapshot{VID: "0403", PID: "6001", Serial: "FT123"}, Message: "D3"})
	m.Add(model.Event{Device: model.DeviceSnapshot{COMPort: "COM53"}, Message: "other"})

	history := m.DeviceHistory(device, 10)
	if len(history) != 2 {
		t.Fatalf("DeviceHistory length = %d, want 2", len(history))
	}
	if history[0].Message != "arrival" || history[1].Message != "D3" {
		t.Fatalf("unexpected history: %#v", history)
	}
}

func TestAppSuppressesEventsForUnmonitoredDevice(t *testing.T) {
	device := model.DeviceSnapshot{
		InstanceID: `USB\VID_0BDA&PID_0129\A`,
		HardwareID: `USB\VID_0BDA&PID_0129`,
	}
	devices := newDeviceTableModel()
	devices.Set([]model.DeviceSnapshot{device})
	if err := devices.SetChecked(0, false); err != nil {
		t.Fatalf("SetChecked returned error: %v", err)
	}

	a := &app{devices: devices}
	if a.isEventMonitored(model.Event{Device: device}) {
		t.Fatalf("device-specific event should be suppressed when target is off")
	}
	if !a.isEventMonitored(model.Event{Type: model.EventInfo, Source: model.SourceApp}) {
		t.Fatalf("app-level event without a device should remain visible")
	}
}
