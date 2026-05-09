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
	cases := map[model.EventType]struct {
		ja string
		en string
	}{
		model.EventSuspectSuspend: {ja: "! Suspend", en: "! Suspend"},
		model.EventPowerD0Exit:    {ja: "! Suspend", en: "! Suspend"},
		model.EventResume:         {ja: "Resume", en: "Resume"},
		model.EventError:          {ja: "エラー", en: "Error"},
		model.EventPnPArrival:     {ja: "PnP +", en: "PnP +"},
		model.EventPnPRemoval:     {ja: "PnP -", en: "PnP -"},
		model.EventInfo:           {ja: "", en: ""},
	}

	for typ, want := range cases {
		if got := eventMark(model.Event{Type: typ}, languageJapanese); got != want.ja {
			t.Fatalf("eventMark(%s, ja) = %q, want %q", typ, got, want.ja)
		}
		if got := eventMark(model.Event{Type: typ}, languageEnglish); got != want.en {
			t.Fatalf("eventMark(%s, en) = %q, want %q", typ, got, want.en)
		}
	}
}

func TestLanguageStringsUseSingleLanguageLabels(t *testing.T) {
	ja := stringsFor(languageJapanese)
	en := stringsFor(languageEnglish)

	if ja.refreshButton != "更新" {
		t.Fatalf("unexpected Japanese refresh label: %q", ja.refreshButton)
	}
	if ja.languageLabel != "Language" {
		t.Fatalf("unexpected Japanese language selector label: %q", ja.languageLabel)
	}
	if en.refreshButton != "Refresh" {
		t.Fatalf("unexpected English refresh label: %q", en.refreshButton)
	}
	if ja.refreshButton == en.refreshButton {
		t.Fatalf("language labels should differ")
	}
}
