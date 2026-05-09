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

func TestDeviceMonitoringAliasesCoverHardwareIDEvents(t *testing.T) {
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
	if m.IsMonitored(hardwareOnlyEventDevice) {
		t.Fatalf("hardware-id alias should inherit the disabled monitoring state")
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

func TestDeviceTableModelShowsStateColumnAndLanguage(t *testing.T) {
	m := newDeviceTableModel()
	device := model.DeviceSnapshot{
		InstanceID:   `USB\VID_0BDA&PID_0129\A`,
		FriendlyName: "USB Reader",
		Present:      true,
		PowerState:   model.PowerD3,
	}

	m.Set([]model.DeviceSnapshot{device})
	if got := m.Value(0, 1); got != "低電力 / Suspend疑い (D3)" {
		t.Fatalf("Japanese state column = %q", got)
	}

	m.SetLanguage(languageEnglish)
	if got := m.Value(0, 1); got != "Low power / suspected suspend (D3)" {
		t.Fatalf("English state column = %q", got)
	}

	if err := m.SetChecked(0, false); err != nil {
		t.Fatalf("SetChecked returned error: %v", err)
	}
	if got := m.Value(0, 1); got != "Monitoring off" {
		t.Fatalf("unchecked state column = %q", got)
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
