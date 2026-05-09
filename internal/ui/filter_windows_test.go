package ui

import (
	"testing"

	"usb-suspend-watch/internal/model"
)

func TestFilterEventsByTypeConfidenceAndQuery(t *testing.T) {
	events := []model.Event{
		{
			Type:       model.EventSuspectSuspend,
			Confidence: model.ConfidenceMedium,
			Source:     model.SourceSetupAPIPoll,
			Device: model.DeviceSnapshot{
				FriendlyName: "USB Composite Device",
				InstanceID:   `USB\VID_1234&PID_ABCD\1`,
				VID:          "1234",
				PID:          "ABCD",
			},
			Message: "device entered a low-power state",
		},
		{
			Type:       model.EventResume,
			Confidence: model.ConfidenceLow,
			Source:     model.SourceSetupAPIPoll,
			Device:     model.DeviceSnapshot{FriendlyName: "Realtek USB Reader"},
			Message:    "device returned to D0",
		},
		{
			Type:       model.EventError,
			Confidence: model.ConfidenceHigh,
			Source:     model.SourceApp,
			Message:    "failed to start helper",
		},
	}

	filtered := filterEvents(events, eventFilter{
		TypeIndex:       1,
		ConfidenceIndex: 1,
		Query:           "vid_1234",
	})

	if len(filtered) != 1 {
		t.Fatalf("expected one suspend-related event, got %d", len(filtered))
	}
	if filtered[0].Type != model.EventSuspectSuspend {
		t.Fatalf("unexpected event type: %s", filtered[0].Type)
	}
}

func TestFilterEventsHighOnlyAndError(t *testing.T) {
	events := []model.Event{
		{Type: model.EventError, Confidence: model.ConfidenceHigh, Message: "first"},
		{Type: model.EventError, Confidence: model.ConfidenceMedium, Message: "second"},
		{Type: model.EventResume, Confidence: model.ConfidenceHigh, Message: "third"},
	}

	filtered := filterEvents(events, eventFilter{TypeIndex: 4, ConfidenceIndex: 2})
	if len(filtered) != 1 {
		t.Fatalf("expected one high-confidence error, got %d", len(filtered))
	}
	if filtered[0].Message != "first" {
		t.Fatalf("unexpected filtered event: %#v", filtered[0])
	}
}

func TestEventMark(t *testing.T) {
	cases := map[model.EventType]string{
		model.EventSuspectSuspend: "! Suspend",
		model.EventPowerD0Exit:    "! Suspend",
		model.EventResume:         "Resume",
		model.EventError:          "Error",
		model.EventPnPArrival:     "PnP +",
		model.EventPnPRemoval:     "PnP -",
		model.EventInfo:           "",
	}

	for typ, want := range cases {
		if got := eventMark(model.Event{Type: typ}); got != want {
			t.Fatalf("eventMark(%s) = %q, want %q", typ, got, want)
		}
	}
}
