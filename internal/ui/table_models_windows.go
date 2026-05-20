package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

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
	case 9:
		return compactParentTree(d)
	default:
		return ""
	}
}

func compactParentTree(d model.DeviceSnapshot) string {
	names := make([]string, 0, len(d.ParentStates)+1)
	for i := len(d.ParentStates) - 1; i >= 0; i-- {
		state := d.ParentStates[i]
		name := state.DisplayName
		if name == "" {
			name = state.InstanceID
		}
		if name == "" {
			name = "parent unknown"
		}
		if state.PowerState != "" && state.PowerState != model.PowerUnknown {
			name += " [" + string(state.PowerState) + "]"
		}
		names = append(names, name)
	}
	deviceName := d.DisplayName()
	if d.PowerState != "" && d.PowerState != model.PowerUnknown {
		deviceName += " [" + string(d.PowerState) + "]"
	}
	names = append(names, deviceName)
	return strings.Join(names, " └ ")
}

func (m *deviceTableModel) Set(items []model.DeviceSnapshot) {
	for _, item := range items {
		for _, key := range deviceMonitorKeys(item) {
			if _, ok := m.monitoredByKey[key]; !ok {
				m.monitoredByKey[key] = true
			}
		}
	}
	if sameDeviceRows(m.items, items) {
		m.items = items
		return
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
		state := text.deviceStateActive + " (D0)"
		if device.ParentLowPowerChildD0 {
			state += " | Parent D3"
		}
		return state
	case model.IsLowPowerState(device.PowerState):
		return text.deviceStateLowPower + " (" + string(device.PowerState) + ")"
	case device.PowerState == "" || device.PowerState == model.PowerUnknown:
		return text.deviceStateUnknown
	default:
		return string(device.PowerState)
	}
}

type adapterGroup struct {
	Name      string
	Score     int
	Reasons   string
	Port      string
	Converter string
	Parent    string
}

type adapterGroupTableModel struct {
	walk.TableModelBase
	items []adapterGroup
}

func newAdapterGroupTableModel() *adapterGroupTableModel {
	return &adapterGroupTableModel{}
}

func (m *adapterGroupTableModel) RowCount() int {
	return len(m.items)
}

func (m *adapterGroupTableModel) Value(row, col int) interface{} {
	g := m.items[row]
	switch col {
	case 0:
		return g.Name
	case 1:
		if g.Score <= 0 {
			return "no same-device evidence"
		}
		if g.Reasons == "" {
			return fmt.Sprintf("%d%%", g.Score)
		}
		return fmt.Sprintf("%d%% | %s", g.Score, g.Reasons)
	case 2:
		return g.Port
	case 3:
		return g.Converter
	case 4:
		return g.Parent
	default:
		return ""
	}
}

func (m *adapterGroupTableModel) Set(devices []model.DeviceSnapshot) {
	m.items = buildAdapterGroups(devices)
	m.PublishRowsReset()
}

func buildAdapterGroups(devices []model.DeviceSnapshot) []adapterGroup {
	byGroup := make(map[string]*adapterGroup)
	order := make([]string, 0)
	for _, device := range devices {
		key := strings.TrimSpace(device.LogicalGroupID)
		if key == "" {
			key = "single:" + strings.ToLower(device.InstanceID)
		}
		group, ok := byGroup[key]
		if !ok {
			name := device.GroupDisplayName
			if name == "" {
				name = device.DisplayName()
			}
			group = &adapterGroup{Name: name, Score: device.DiagnosticScore, Reasons: strings.Join(device.DiagnosticReasons, " | ")}
			byGroup[key] = group
			order = append(order, key)
		}
		if device.DiagnosticScore > group.Score {
			group.Score = device.DiagnosticScore
			group.Reasons = strings.Join(device.DiagnosticReasons, " | ")
		}
		if device.ParentInstanceID != "" && group.Parent == "" {
			group.Parent = device.ParentInstanceID
		}
		switch device.RelationRole {
		case "port":
			group.Port = compactGroupMember(group.Port, device.DisplayName())
		case "converter":
			group.Converter = compactGroupMember(group.Converter, device.DisplayName())
		default:
			if device.COMPort != "" {
				group.Port = compactGroupMember(group.Port, device.DisplayName())
			} else {
				group.Converter = compactGroupMember(group.Converter, device.DisplayName())
			}
		}
	}
	out := make([]adapterGroup, 0, len(order))
	for _, key := range order {
		out = append(out, *byGroup[key])
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func compactGroupMember(existing, next string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return existing
	}
	if existing == "" {
		return next
	}
	if strings.Contains(existing, next) {
		return existing
	}
	return existing + " | " + next
}

func hasDeviceIdentity(device model.DeviceSnapshot) bool {
	return strings.TrimSpace(device.InstanceID) != "" ||
		strings.TrimSpace(device.HardwareID) != "" ||
		strings.TrimSpace(device.FriendlyName) != "" ||
		strings.TrimSpace(device.Description) != ""
}

func deviceMonitorKeys(device model.DeviceSnapshot) []string {
	candidates := []string{device.LogicalGroupID}
	if strings.TrimSpace(device.Serial) != "" {
		if strings.TrimSpace(device.VID) != "" || strings.TrimSpace(device.PID) != "" {
			candidates = append(candidates, device.VIDPID()+" serial="+device.Serial)
		}
	}
	candidates = append(candidates, device.RelatedInstanceIDs...)
	candidates = append(candidates,
		device.InstanceID,
		device.COMPort,
	)
	if !hasSpecificMonitorIdentity(device) {
		candidates = append(candidates, device.HardwareID)
		if displayName := device.DisplayName(); displayName != "(unknown USB device)" {
			candidates = append(candidates, displayName)
		}
		candidates = append(candidates, device.VIDPID())
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

func hasSpecificMonitorIdentity(device model.DeviceSnapshot) bool {
	if strings.TrimSpace(device.LogicalGroupID) != "" ||
		strings.TrimSpace(device.InstanceID) != "" ||
		strings.TrimSpace(device.COMPort) != "" ||
		len(device.RelatedInstanceIDs) > 0 {
		return true
	}
	return strings.TrimSpace(device.Serial) != "" &&
		(strings.TrimSpace(device.VID) != "" || strings.TrimSpace(device.PID) != "")
}

func sameDeviceRows(a, b []model.DeviceSnapshot) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if deviceRowSignature(a[i]) != deviceRowSignature(b[i]) {
			return false
		}
	}
	return true
}

func deviceRowSignature(d model.DeviceSnapshot) string {
	return strings.Join([]string{
		d.DisplayName(),
		deviceCurrentState(d, true, languageEnglish),
		d.VIDPID(),
		string(d.PowerState),
		d.Enumerator,
		d.COMPort,
		d.Location,
		d.ConnectedAt.Format(time.RFC3339Nano),
		d.LastSeen.Format(time.RFC3339Nano),
		compactParentTree(d),
	}, "\x00")
}

func sameDeviceForHistory(a, b model.DeviceSnapshot) bool {
	if strings.TrimSpace(a.InstanceID) != "" && strings.EqualFold(a.InstanceID, b.InstanceID) {
		return true
	}
	if strings.TrimSpace(a.LogicalGroupID) != "" && strings.EqualFold(a.LogicalGroupID, b.LogicalGroupID) {
		return true
	}
	if containsFold(a.RelatedInstanceIDs, b.InstanceID) || containsFold(b.RelatedInstanceIDs, a.InstanceID) {
		return true
	}
	if strings.TrimSpace(a.Serial) != "" &&
		strings.EqualFold(a.Serial, b.Serial) &&
		strings.EqualFold(a.VID, b.VID) &&
		strings.EqualFold(a.PID, b.PID) {
		return true
	}
	if strings.TrimSpace(a.Serial) != "" &&
		strings.TrimSpace(b.Serial) != "" &&
		!strings.EqualFold(a.Serial, b.Serial) {
		return false
	}
	if strings.TrimSpace(a.COMPort) != "" && strings.EqualFold(a.COMPort, b.COMPort) {
		return true
	}
	return false
}

func sameDeviceForSelection(a, b model.DeviceSnapshot) bool {
	if strings.TrimSpace(a.InstanceID) != "" && strings.EqualFold(a.InstanceID, b.InstanceID) {
		return true
	}
	if strings.TrimSpace(a.LogicalGroupID) != "" && strings.EqualFold(a.LogicalGroupID, b.LogicalGroupID) {
		return true
	}
	if containsFold(a.RelatedInstanceIDs, b.InstanceID) || containsFold(b.RelatedInstanceIDs, a.InstanceID) {
		return true
	}
	if strings.TrimSpace(a.Serial) != "" &&
		strings.EqualFold(a.Serial, b.Serial) &&
		strings.EqualFold(a.VID, b.VID) &&
		strings.EqualFold(a.PID, b.PID) {
		return true
	}
	if strings.TrimSpace(a.Serial) == "" &&
		strings.TrimSpace(b.Serial) == "" &&
		strings.TrimSpace(a.COMPort) != "" &&
		strings.EqualFold(a.COMPort, b.COMPort) {
		return true
	}
	return false
}

func containsFold(values []string, target string) bool {
	if strings.TrimSpace(target) == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
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
	oldVisibleLen := len(m.visible)
	m.items = append(m.items, event)
	trimmed := false
	if m.limit > 0 && len(m.items) > m.limit {
		m.items = m.items[len(m.items)-m.limit:]
		trimmed = true
	}
	nextVisible := filterEvents(m.items, m.filter)
	switch {
	case trimmed:
		m.visible = nextVisible
		m.PublishRowsReset()
	case len(nextVisible) == oldVisibleLen:
		m.visible = nextVisible
	case len(nextVisible) == oldVisibleLen+1:
		m.visible = nextVisible
		m.PublishRowsInserted(oldVisibleLen, oldVisibleLen)
	default:
		m.visible = nextVisible
		m.PublishRowsReset()
	}
}

func (m *eventTableModel) Set(events []model.Event) {
	m.items = append([]model.Event(nil), events...)
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
	matches := make([]bool, len(m.items))
	for i, event := range m.items {
		matches[i] = sameDeviceForHistory(device, event.Device)
	}
	out := make([]model.Event, 0, limit)
	for i := len(m.items) - 1; i >= 0; i-- {
		event := m.items[i]
		if !matches[i] && !isSystemPowerNearDeviceEvent(m.items, matches, i, 30*time.Second) {
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

func (m *eventTableModel) WakeCorrelation(t time.Time, window time.Duration) []model.Event {
	if m == nil || t.IsZero() || window <= 0 {
		return nil
	}
	var out []model.Event
	for _, event := range m.items {
		if !isWakeCorrelationEvent(event) {
			continue
		}
		delta := event.Time.Sub(t)
		if delta < 0 {
			delta = -delta
		}
		if delta <= window {
			out = append(out, event)
		}
	}
	return out
}

func isSystemPowerNearDeviceEvent(events []model.Event, matches []bool, index int, window time.Duration) bool {
	if index < 0 || index >= len(events) || !isSystemPowerEvent(events[index]) {
		return false
	}
	for i, matched := range matches {
		if !matched {
			continue
		}
		delta := events[index].Time.Sub(events[i].Time)
		if delta < 0 {
			delta = -delta
		}
		if delta <= window {
			return true
		}
	}
	return false
}

func isSystemPowerEvent(event model.Event) bool {
	return event.Type == model.EventSystemSleep || event.Type == model.EventSystemWake
}

func isWakeCorrelationEvent(event model.Event) bool {
	switch event.Type {
	case model.EventPnPArrival, model.EventPnPRemoval,
		model.EventPowerD0Exit, model.EventPowerD0Entry,
		model.EventSuspectSuspend, model.EventResume:
		return true
	default:
		return false
	}
}

func isUSBChangeTimelineEvent(event model.Event) bool {
	switch event.Type {
	case model.EventPnPArrival,
		model.EventPnPRemoval,
		model.EventPowerD0Exit,
		model.EventPowerD0Entry,
		model.EventSuspectSuspend,
		model.EventResume,
		model.EventIdleNotification,
		model.EventSystemSleep,
		model.EventSystemWake:
		return true
	default:
		return false
	}
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
	for _, event := range m.visible {
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
