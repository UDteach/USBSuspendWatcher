package monitor

import (
	"context"
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
			p.send(model.Event{
				Time:       d.LastSeen,
				Type:       model.EventPnPArrival,
				Source:     model.SourceSetupAPIPoll,
				Confidence: model.ConfidenceHigh,
				Device:     d,
				Message:    "USB device appeared in SetupAPI snapshot at " + d.ConnectedAt.Format(time.RFC3339),
				Raw:        model.DeviceEvidenceRaw(d),
			})
			continue
		}
		for _, event := range model.NormalizePowerTransition(old, d) {
			p.send(event)
		}
	}

	for id, old := range prev {
		if _, ok := current[id]; ok {
			continue
		}
		old.Present = false
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
	}
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
