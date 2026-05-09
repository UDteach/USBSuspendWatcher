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
