package model

import (
	"testing"
	"time"
)

func TestNormalizePowerTransitionD0Exit(t *testing.T) {
	prev := DeviceSnapshot{InstanceID: `USB\VID_1234&PID_ABCD\1`, PowerState: PowerD0}
	curr := prev
	curr.PowerState = PowerD3
	curr.LastSeen = time.Unix(10, 0)

	events := NormalizePowerTransition(prev, curr)
	if len(events) != 3 {
		t.Fatalf("expected three events, got %d", len(events))
	}
	if events[0].Type != EventDStateTransition || events[1].Type != EventPowerD0Exit || events[2].Type != EventSuspectSuspend {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestNormalizePowerTransitionD0Entry(t *testing.T) {
	prev := DeviceSnapshot{InstanceID: `USB\VID_1234&PID_ABCD\1`, PowerState: PowerD3}
	curr := prev
	curr.PowerState = PowerD0
	curr.LastSeen = time.Unix(20, 0)

	events := NormalizePowerTransition(prev, curr)
	if len(events) != 3 {
		t.Fatalf("expected three events, got %d", len(events))
	}
	if events[0].Type != EventDStateTransition || events[1].Type != EventPowerD0Entry || events[2].Type != EventResume {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestPhysicalRemovalIsNotSuspend(t *testing.T) {
	prev := DeviceSnapshot{InstanceID: `USB\VID_1234&PID_ABCD\1`, PowerState: PowerD0}
	curr := prev
	curr.Present = false
	curr.PowerState = PowerUnknown

	events := NormalizePowerTransition(prev, curr)
	if len(events) != 0 {
		t.Fatalf("expected no suspend transition for removal-like snapshot, got %#v", events)
	}
}

func TestNormalizeETWUSBHUB3(t *testing.T) {
	event := NormalizeETW(ETWRecord{
		Provider: "Microsoft-Windows-USB-USBHUB3",
		EventID:  54,
		Properties: map[string]string{
			"DeviceId": `USB\VID_045E&PID_1234\ABC`,
		},
	})

	if event.Type != EventPowerD0Exit {
		t.Fatalf("expected D0 exit, got %s", event.Type)
	}
	if event.Confidence != ConfidenceHigh {
		t.Fatalf("expected high confidence, got %s", event.Confidence)
	}
	if event.Device.VID != "045E" || event.Device.PID != "1234" {
		t.Fatalf("unexpected ids: %#v", event.Device)
	}
}
