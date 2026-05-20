package ui

import (
	"strings"

	"github.com/lxn/walk"

	"usb-suspend-watch/internal/model"
)

type deviceTableModel struct {
	walk.TableModelBase
	items            []model.DeviceSnapshot
	monitoredByKey   map[string]bool
	language         displayLanguage
	onMonitorChanged func(device model.DeviceSnapshot, monitored bool)
}

func newDeviceTableModel() *deviceTableModel {
	return &deviceTableModel{monitoredByKey: make(map[string]bool), language: languageJapanese}
}

func (m *deviceTableModel) RowCount() int {
	return len(m.items)
}

func (m *deviceTableModel) Value(row, col int) interface{} {
	d := m.items[row]
	switch col {
	case 0:
		return d.DisplayName()
	case 1:
		return deviceCurrentState(d, m.IsMonitored(d), m.language)
	case 2:
		return d.VIDPID()
	case 3:
		return string(d.PowerState)
	case 4:
		return d.Enumerator
	case 5:
		return d.COMPort
	case 6:
		return d.Location
	case 7:
		if d.ConnectedAt.IsZero() {
			return ""
		}
		return d.ConnectedAt.Format("15:04:05")
	case 8:
		if d.LastSeen.IsZero() {
			return ""
		}
		return d.LastSeen.Format("15:04:05")
	default:
		return ""
	}
}

func (m *deviceTableModel) Set(items []model.DeviceSnapshot) {
	for _, item := range items {
		for _, key := range deviceMonitorKeys(item) {
			if _, ok := m.monitoredByKey[key]; !ok {
				m.monitoredByKey[key] = true
			}
		}
	}
	m.items = items
	m.PublishRowsReset()
}

func (m *deviceTableModel) SetLanguage(language displayLanguage) {
	m.language = language
	m.PublishRowsReset()
}

func (m *deviceTableModel) Item(row int) (model.DeviceSnapshot, bool) {
	if row < 0 || row >= len(m.items) {
		return model.DeviceSnapshot{}, false
	}
	return m.items[row], true
}

func (m *deviceTableModel) All() []model.DeviceSnapshot {
	out := make([]model.DeviceSnapshot, len(m.items))
	copy(out, m.items)
	return out
}

func (m *deviceTableModel) Checked(row int) bool {
	if row < 0 || row >= len(m.items) {
		return false
	}
	return m.IsMonitored(m.items[row])
}

func (m *deviceTableModel) SetChecked(row int, checked bool) error {
	if row < 0 || row >= len(m.items) {
		return nil
	}
	device := m.items[row]
	for _, key := range deviceMonitorKeys(device) {
		m.monitoredByKey[key] = checked
	}
	m.PublishRowChanged(row)
	if m.onMonitorChanged != nil {
		m.onMonitorChanged(device, checked)
	}
	return nil
}

func (m *deviceTableModel) IsMonitored(device model.DeviceSnapshot) bool {
	keys := deviceMonitorKeys(device)
	if len(keys) == 0 {
		return true
	}
	for _, key := range keys {
		if monitored, ok := m.monitoredByKey[key]; ok {
			return monitored
		}
	}
	return true
}

func (m *deviceTableModel) IsMonitoredKey(key string) bool {
	if key == "" {
		return true
	}
	monitored, ok := m.monitoredByKey[key]
	if !ok {
		return true
	}
	return monitored
}

func (m *deviceTableModel) MonitoredCount() int {
	count := 0
	for _, device := range m.items {
		if m.IsMonitored(device) {
			count++
		}
	}
	return count
}

func deviceCurrentState(device model.DeviceSnapshot, monitored bool, language displayLanguage) string {
	text := stringsFor(language)
	if !monitored {
		return text.deviceStateMonitoringOff
	}
	if !device.Present {
		if !hasDeviceIdentity(device) {
			return text.deviceStateUnknown
		}
		return text.deviceStateRemoved
	}
	switch {
	case device.PowerState == model.PowerD0:
		return text.deviceStateActive + " (D0)"
	case model.IsLowPowerState(device.PowerState):
		return text.deviceStateLowPower + " (" + string(device.PowerState) + ")"
	case device.PowerState == "" || device.PowerState == model.PowerUnknown:
		return text.deviceStateUnknown
	default:
		return string(device.PowerState)
	}
}

func hasDeviceIdentity(device model.DeviceSnapshot) bool {
	return strings.TrimSpace(device.InstanceID) != "" ||
		strings.TrimSpace(device.HardwareID) != "" ||
		strings.TrimSpace(device.FriendlyName) != "" ||
		strings.TrimSpace(device.Description) != ""
}

func deviceMonitorKeys(device model.DeviceSnapshot) []string {
	candidates := []string{
		device.InstanceID,
		device.HardwareID,
		device.VIDPID(),
		device.COMPort,
	}
	if displayName := device.DisplayName(); displayName != "(unknown USB device)" {
		candidates = append(candidates, displayName)
	}
	keys := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		key := strings.ToLower(strings.TrimSpace(candidate))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func sameDeviceForHistory(a, b model.DeviceSnapshot) bool {
	if strings.TrimSpace(a.InstanceID) != "" && strings.EqualFold(a.InstanceID, b.InstanceID) {
		return true
	}
	if strings.TrimSpace(a.COMPort) != "" && strings.EqualFold(a.COMPort, b.COMPort) {
		return true
	}
	if strings.TrimSpace(a.Serial) != "" &&
		strings.EqualFold(a.Serial, b.Serial) &&
		strings.EqualFold(a.VID, b.VID) &&
		strings.EqualFold(a.PID, b.PID) {
		return true
	}
	return false
}

type eventTableModel struct {
	walk.TableModelBase
	items    []model.Event
	visible  []model.Event
	filter   eventFilter
	language displayLanguage
	limit    int
}

func newEventTableModel(limit int) *eventTableModel {
	return &eventTableModel{limit: limit, language: languageJapanese}
}

func (m *eventTableModel) RowCount() int {
	return len(m.visible)
}

func (m *eventTableModel) Value(row, col int) interface{} {
	e := m.visible[row]
	switch col {
	case 0:
		return eventMark(e, m.language)
	case 1:
		return e.Time.Format("2006-01-02 15:04:05")
	case 2:
		return string(e.Type)
	case 3:
		return string(e.Confidence)
	case 4:
		return string(e.Source)
	case 5:
		return e.Device.DisplayName()
	case 6:
		return e.Message
	default:
		return ""
	}
}

func (m *eventTableModel) Add(event model.Event) {
	m.items = append(m.items, event)
	if m.limit > 0 && len(m.items) > m.limit {
		m.items = m.items[len(m.items)-m.limit:]
	}
	m.visible = filterEvents(m.items, m.filter)
	m.PublishRowsReset()
}

func (m *eventTableModel) Item(row int) (model.Event, bool) {
	if row < 0 || row >= len(m.visible) {
		return model.Event{}, false
	}
	return m.visible[row], true
}

func (m *eventTableModel) All() []model.Event {
	out := make([]model.Event, len(m.visible))
	copy(out, m.visible)
	return out
}

func (m *eventTableModel) DeviceHistory(device model.DeviceSnapshot, limit int) []model.Event {
	if m == nil {
		return nil
	}
	out := make([]model.Event, 0, limit)
	for i := len(m.items) - 1; i >= 0; i-- {
		event := m.items[i]
		if !sameDeviceForHistory(device, event.Device) {
			continue
		}
		out = append(out, event)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func (m *eventTableModel) SetFilter(filter eventFilter) {
	m.filter = filter
	m.visible = filterEvents(m.items, m.filter)
	m.PublishRowsReset()
}

func (m *eventTableModel) SetLanguage(language displayLanguage) {
	m.language = language
	m.PublishRowsReset()
}

func (m *eventTableModel) Stats() eventStats {
	var stats eventStats
	for _, event := range m.items {
		switch event.Type {
		case model.EventSuspectSuspend:
			stats.SuspectedSuspend++
		case model.EventResume:
			stats.Resume++
		case model.EventError:
			stats.Error++
		}
	}
	return stats
}

type eventStats struct {
	SuspectedSuspend int
	Resume           int
	Error            int
}
