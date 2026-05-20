package monitor

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"usb-suspend-watch/internal/model"
	"usb-suspend-watch/internal/usb"
)

type Poller struct {
	interval time.Duration
	events   chan model.Event
	refresh  chan struct{}

	mu       sync.RWMutex
	devices  []model.DeviceSnapshot
	previous map[string]model.DeviceSnapshot
	ready    bool
}

func NewPoller(interval time.Duration) *Poller {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &Poller{
		interval: interval,
		events:   make(chan model.Event, 256),
		refresh:  make(chan struct{}, 1),
		previous: make(map[string]model.DeviceSnapshot),
	}
}

func (p *Poller) Events() <-chan model.Event {
	return p.events
}

func (p *Poller) Snapshot() []model.DeviceSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]model.DeviceSnapshot, len(p.devices))
	copy(out, p.devices)
	return out
}

func (p *Poller) RefreshNow() {
	select {
	case p.refresh <- struct{}{}:
	default:
	}
}

func (p *Poller) Prime() {
	p.scan(false)
}

func (p *Poller) Run(ctx context.Context) {
	defer close(p.events)
	if !p.isReady() {
		p.scan(false)
	}
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.scan(true)
		case <-p.refresh:
			p.scan(true)
		}
	}
}

func (p *Poller) isReady() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ready
}

func (p *Poller) scan(emit bool) {
	devices, err := usb.ListPresentDevices()
	if err != nil {
		p.send(model.Event{
			Time:       time.Now(),
			Type:       model.EventError,
			Source:     model.SourceSetupAPIPoll,
			Confidence: model.ConfidenceLow,
			Message:    err.Error(),
		})
		return
	}

	p.mu.RLock()
	prev := p.previous
	wasReady := p.ready
	p.mu.RUnlock()

	current := make(map[string]model.DeviceSnapshot, len(devices))
	for i, d := range devices {
		d.SessionObserved = true
		if old, ok := prev[d.InstanceID]; ok {
			d.ConnectedAt = old.ConnectedAt
			d.LastChanged = old.LastChanged
			if d.ConnectedAt.IsZero() {
				d.ConnectedAt = old.LastSeen
			}
			if deviceChanged(old, d) {
				d.LastChanged = d.LastSeen
			}
		} else {
			d.ConnectedAt = d.LastSeen
			d.LastChanged = d.LastSeen
		}
		devices[i] = d
		current[d.InstanceID] = d
	}

	p.mu.Lock()
	p.previous = current
	p.devices = devices
	p.ready = true
	p.mu.Unlock()

	if !emit || !wasReady {
		return
	}

	for id, d := range current {
		old, ok := prev[id]
		if !ok {
			message := "USB device appeared in SetupAPI snapshot at " + d.ConnectedAt.Format(time.RFC3339)
			if d.ParentLowPowerChildD0 {
				message += "; parent hub/device is low power while child reports D0"
			}
			p.send(model.Event{
				Time:       d.LastSeen,
				Type:       model.EventPnPArrival,
				Source:     model.SourceSetupAPIPoll,
				Confidence: model.ConfidenceHigh,
				Device:     d,
				Message:    message,
				Raw:        model.DeviceEvidenceRaw(d),
			})
			if d.ParentLowPowerChildD0 {
				p.send(parentMismatchEvent(d, "parent hub/device is low power while newly observed child reports D0"))
			}
			if d.ProblemCode != 0 {
				p.send(problemCodeEvent(d, 0))
			}
			continue
		}
		for _, event := range model.NormalizePowerTransition(old, d) {
			p.send(event)
		}
		if !old.ParentLowPowerChildD0 && d.ParentLowPowerChildD0 {
			p.send(parentMismatchEvent(d, "parent hub/device became low power while child reports D0"))
		}
		if old.ProblemCode != d.ProblemCode && d.ProblemCode != 0 {
			p.send(problemCodeEvent(d, old.ProblemCode))
		}
		if old.Status != d.Status || old.StatusFlags != d.StatusFlags {
			p.send(statusChangedEvent(old, d))
		}
	}

	for id, old := range prev {
		if _, ok := current[id]; ok {
			continue
		}
		old.Present = false
		old.SessionObserved = true
		old.LastSeen = time.Now()
		p.send(model.Event{
			Time:       old.LastSeen,
			Type:       model.EventPnPRemoval,
			Source:     model.SourceSetupAPIPoll,
			Confidence: model.ConfidenceHigh,
			Device:     old,
			Message:    "USB device disappeared from SetupAPI snapshot",
			Raw:        model.DeviceEvidenceRaw(old),
		})
		p.send(model.Event{
			Time:       old.LastSeen,
			Type:       model.EventDeviceMissing,
			Source:     model.SourceSetupAPIPoll,
			Confidence: model.ConfidenceHigh,
			Device:     old,
			Message:    "device missing from current SetupAPI snapshot",
			Raw:        model.DeviceEvidenceRaw(old),
		})
		if next, reason, ok := findReenumeratedDevice(old, current); ok {
			raw := model.DeviceEvidenceRaw(next)
			raw["previous_instance_id"] = old.InstanceID
			raw["current_instance_id"] = next.InstanceID
			raw["reenumeration_reason"] = reason
			p.send(model.Event{
				Time:       next.LastSeen,
				Type:       model.EventDeviceReenum,
				Source:     model.SourceSetupAPIPoll,
				Confidence: model.ConfidenceMedium,
				Device:     next,
				Message:    "device re-enumerated candidate: " + reason,
				Raw:        raw,
			})
		}
	}
}

func parentMismatchEvent(d model.DeviceSnapshot, message string) model.Event {
	return model.Event{
		Time:       d.LastSeen,
		Type:       model.EventParentMismatch,
		Source:     model.SourceSetupAPIPoll,
		Confidence: model.ConfidenceHigh,
		Device:     d,
		Message:    message,
		Raw:        model.DeviceEvidenceRaw(d),
	}
}

func problemCodeEvent(d model.DeviceSnapshot, previous uint32) model.Event {
	raw := model.DeviceEvidenceRaw(d)
	raw["previous_problem_code"] = fmtUint32(previous)
	return model.Event{
		Time:       d.LastSeen,
		Type:       model.EventProblemCode,
		Source:     model.SourceSetupAPIPoll,
		Confidence: model.ConfidenceHigh,
		Device:     d,
		Message:    "Config Manager problem code detected",
		Raw:        raw,
	}
}

func statusChangedEvent(old, d model.DeviceSnapshot) model.Event {
	raw := model.DeviceEvidenceRaw(d)
	raw["previous_status"] = old.Status
	raw["previous_status_flags"] = fmtUint32(old.StatusFlags)
	return model.Event{
		Time:       d.LastSeen,
		Type:       model.EventStatusChanged,
		Source:     model.SourceSetupAPIPoll,
		Confidence: model.ConfidenceMedium,
		Device:     d,
		Message:    "device Config Manager status changed",
		Raw:        raw,
	}
}

func findReenumeratedDevice(old model.DeviceSnapshot, current map[string]model.DeviceSnapshot) (model.DeviceSnapshot, string, bool) {
	for _, d := range current {
		if strings.EqualFold(d.InstanceID, old.InstanceID) {
			continue
		}
		if strings.TrimSpace(old.ContainerID) != "" && strings.EqualFold(old.ContainerID, d.ContainerID) {
			return d, "same ContainerId with changed InstanceId", true
		}
		if strings.TrimSpace(old.Serial) != "" &&
			strings.EqualFold(old.Serial, d.Serial) &&
			strings.EqualFold(old.VID, d.VID) &&
			strings.EqualFold(old.PID, d.PID) {
			return d, "same VID/PID + Serial with changed InstanceId", true
		}
		if len(old.LocationPaths) > 0 && sameAnyFold(old.LocationPaths, d.LocationPaths) {
			return d, "same LocationPath with changed InstanceId", true
		}
	}
	return model.DeviceSnapshot{}, "", false
}

func sameAnyFold(a, b []string) bool {
	for _, av := range a {
		for _, bv := range b {
			if strings.TrimSpace(av) != "" && strings.EqualFold(av, bv) {
				return true
			}
		}
	}
	return false
}

func fmtUint32(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}

func deviceChanged(old, current model.DeviceSnapshot) bool {
	return old.PowerState != current.PowerState ||
		old.Present != current.Present ||
		old.COMPort != current.COMPort ||
		old.ParentInstanceID != current.ParentInstanceID ||
		old.Location != current.Location
}

func (p *Poller) send(event model.Event) {
	select {
	case p.events <- event:
	default:
	}
}
