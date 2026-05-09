package ui

import (
	"github.com/lxn/walk"

	"usb-suspend-watch/internal/model"
)

type deviceTableModel struct {
	walk.TableModelBase
	items []model.DeviceSnapshot
}

func newDeviceTableModel() *deviceTableModel {
	return &deviceTableModel{}
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
		return d.VIDPID()
	case 2:
		return string(d.PowerState)
	case 3:
		return d.Enumerator
	case 4:
		return d.Location
	case 5:
		if d.LastSeen.IsZero() {
			return ""
		}
		return d.LastSeen.Format("15:04:05")
	default:
		return ""
	}
}

func (m *deviceTableModel) Set(items []model.DeviceSnapshot) {
	m.items = items
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

type eventTableModel struct {
	walk.TableModelBase
	items   []model.Event
	visible []model.Event
	filter  eventFilter
	limit   int
}

func newEventTableModel(limit int) *eventTableModel {
	return &eventTableModel{limit: limit}
}

func (m *eventTableModel) RowCount() int {
	return len(m.visible)
}

func (m *eventTableModel) Value(row, col int) interface{} {
	e := m.visible[row]
	switch col {
	case 0:
		return eventMark(e)
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

func (m *eventTableModel) SetFilter(filter eventFilter) {
	m.filter = filter
	m.visible = filterEvents(m.items, m.filter)
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
