package ui

import (
	"errors"
	"testing"
	"time"

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

func TestPowercfgLastWakeRawFromOutput(t *testing.T) {
	raw := powercfgLastWakeRawFromOutput([]byte("Wake History Count - 1\r\n"), nil)
	if raw["lastwake"] != "Wake History Count - 1" {
		t.Fatalf("lastwake output not captured: %#v", raw)
	}
	if raw["lastwake_source"] != string(model.SourcePowercfgLastWake) {
		t.Fatalf("lastwake source not captured: %#v", raw)
	}

	raw = powercfgLastWakeRawFromOutput(nil, errors.New("powercfg failed"))
	if raw["lastwake_error"] != "powercfg failed" {
		t.Fatalf("lastwake error not captured: %#v", raw)
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

func TestFTDICOMOnlyTargetFilter(t *testing.T) {
	ftdi := model.Event{
		Type:       model.EventPnPArrival,
		Confidence: model.ConfidenceHigh,
		Device: model.DeviceSnapshot{
			FriendlyName: "USB Serial Port (COM52)",
			VID:          "0403",
			PID:          "6001",
			COMPort:      "COM52",
		},
	}
	other := model.Event{
		Type:       model.EventPnPArrival,
		Confidence: model.ConfidenceHigh,
		Device: model.DeviceSnapshot{
			FriendlyName: "USB Keyboard",
			VID:          "1234",
			PID:          "5678",
		},
	}
	converter := model.Event{
		Type:       model.EventPnPArrival,
		Confidence: model.ConfidenceHigh,
		Device: model.DeviceSnapshot{
			FriendlyName:       "USB Serial Converter",
			VID:                "0403",
			PID:                "6001",
			RelationRole:       "converter",
			LogicalGroupID:     "vidpid-serial:0403:6001:FT123",
			RelatedInstanceIDs: []string{`FTDIPORT\VID_0403+PID_6001+FT123\0000`},
		},
	}

	filtered := filterEvents([]model.Event{ftdi, other, converter}, eventFilter{TargetIndex: 1})
	if len(filtered) != 2 || filtered[0].Device.COMPort != "COM52" || filtered[1].Device.RelationRole != "converter" {
		t.Fatalf("FTDI COM filter returned %#v", filtered)
	}

	devices := filterDevicesForTarget([]model.DeviceSnapshot{ftdi.Device, other.Device, converter.Device}, 1)
	if len(devices) != 2 || devices[0].COMPort != "COM52" || devices[1].RelationRole != "converter" {
		t.Fatalf("device target filter returned %#v", devices)
	}
}

func TestPowercfgLastWakeEventSearchesRawData(t *testing.T) {
	event := model.Event{
		Type:   model.EventSystemWake,
		Source: model.SourcePowerBroadcast,
		Raw: map[string]string{
			"lastwake": "Wake History Count - 1",
		},
	}
	if !eventMatchesFilter(event, eventFilter{Query: "wake history"}) {
		t.Fatalf("lastwake raw output should be searchable")
	}
}

func TestUSBChangeTimelineEventSelection(t *testing.T) {
	if !isUSBChangeTimelineEvent(model.Event{Type: model.EventPowerD0Exit}) {
		t.Fatalf("D0 exit should be shown in USB changes")
	}
	if !isUSBChangeTimelineEvent(model.Event{Type: model.EventSystemWake}) {
		t.Fatalf("system wake should be shown in USB changes")
	}
	if isUSBChangeTimelineEvent(model.Event{Type: model.EventInfo}) {
		t.Fatalf("generic info should not be shown in USB changes")
	}
}

func TestFilterEventsDisplayLevelSuppressesInfoByDefault(t *testing.T) {
	events := []model.Event{
		{Type: model.EventInfo, Confidence: model.ConfidenceHigh, Message: "noisy ETW state machine event"},
		{Type: model.EventPnPArrival, Confidence: model.ConfidenceMedium, Message: "device arrival"},
		{Type: model.EventPowerD0Exit, Confidence: model.ConfidenceHigh, Message: "D0 exit"},
	}

	filtered := filterEvents(events, eventFilter{})
	if len(filtered) != 2 {
		t.Fatalf("default display level should hide only info events, got %d", len(filtered))
	}
	for _, event := range filtered {
		if event.Type == model.EventInfo {
			t.Fatalf("info event should be hidden by default: %#v", event)
		}
	}

	important := filterEvents(events, eventFilter{LevelIndex: 1})
	if len(important) != 1 || important[0].Type != model.EventPowerD0Exit {
		t.Fatalf("important-only display level returned %#v", important)
	}

	all := filterEvents(events, eventFilter{LevelIndex: 2})
	if len(all) != len(events) {
		t.Fatalf("all display level returned %d events, want %d", len(all), len(events))
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

func TestTerminalETWHelperEvents(t *testing.T) {
	cases := []struct {
		name  string
		event model.Event
		want  bool
	}{
		{
			name:  "helper startup error",
			event: model.Event{Type: model.EventError, Source: model.SourceApp, Message: "start ETW session: access denied"},
			want:  true,
		},
		{
			name:  "parent exited",
			event: model.Event{Type: model.EventInfo, Source: model.SourceApp, Message: "ETW helper parent process exited"},
			want:  true,
		},
		{
			name:  "stop file",
			event: model.Event{Type: model.EventInfo, Source: model.SourceApp, Message: "ETW helper stop file detected"},
			want:  true,
		},
		{
			name:  "running",
			event: model.Event{Type: model.EventInfo, Source: model.SourceApp, Message: "ETW helper running"},
			want:  false,
		},
		{
			name:  "device error",
			event: model.Event{Type: model.EventError, Source: model.SourceSetupAPIPoll, Message: "SetupAPI failed"},
			want:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTerminalETWHelperEvent(tc.event); got != tc.want {
				t.Fatalf("isTerminalETWHelperEvent() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestETWHelperStartupEvents(t *testing.T) {
	cases := []struct {
		name  string
		event model.Event
		want  bool
	}{
		{
			name:  "helper starting",
			event: model.Event{Type: model.EventInfo, Source: model.SourceApp, Message: "ETW helper starting"},
			want:  true,
		},
		{
			name:  "provider enabled",
			event: model.Event{Type: model.EventInfo, Source: model.SourceApp, Message: "ETW provider enabled: Microsoft-Windows-USB-USBHUB3"},
			want:  true,
		},
		{
			name:  "provider unavailable",
			event: model.Event{Type: model.EventInfo, Source: model.SourceApp, Message: "ETW provider unavailable: Microsoft-Windows-USB-UCX: access denied"},
			want:  true,
		},
		{
			name:  "helper running",
			event: model.Event{Type: model.EventInfo, Source: model.SourceApp, Message: "ETW helper running"},
			want:  true,
		},
		{
			name:  "helper error",
			event: model.Event{Type: model.EventError, Source: model.SourceApp, Message: "start ETW session: access denied"},
			want:  true,
		},
		{
			name:  "setupapi event",
			event: model.Event{Type: model.EventError, Source: model.SourceSetupAPIPoll, Message: "SetupAPI failed"},
			want:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isETWHelperStartupEvent(tc.event); got != tc.want {
				t.Fatalf("isETWHelperStartupEvent() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestETWStartupTimeout(t *testing.T) {
	startedAt := time.Unix(100, 0)
	if etwStartupHasTimedOut(false, startedAt, startedAt.Add(etwStartupTimeout-time.Second)) {
		t.Fatalf("timeout fired too early")
	}
	if !etwStartupHasTimedOut(false, startedAt, startedAt.Add(etwStartupTimeout)) {
		t.Fatalf("timeout should fire at threshold")
	}
	if etwStartupHasTimedOut(true, startedAt, startedAt.Add(10*etwStartupTimeout)) {
		t.Fatalf("startup already seen should not time out")
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
	if len(ja.deviceColumnTitles) != 9 || ja.deviceColumnTitles[1] != "状態" || ja.deviceColumnTitles[7] != "接続時刻" {
		t.Fatalf("Japanese device columns should include a state column: %#v", ja.deviceColumnTitles)
	}
	if len(en.deviceColumnTitles) != 9 || en.deviceColumnTitles[1] != "State" || en.deviceColumnTitles[7] != "Connected" {
		t.Fatalf("English device columns should include a state column: %#v", en.deviceColumnTitles)
	}
	if len(ja.levelOptions) != 3 || ja.levelOptions[0] != "Info以外" {
		t.Fatalf("Japanese level options should default to no-info: %#v", ja.levelOptions)
	}
	if len(en.targetOptions) != 2 || en.targetOptions[1] != "FTDI COM only" {
		t.Fatalf("English target options should include FTDI filter: %#v", en.targetOptions)
	}
	if len(en.levelOptions) != 3 || en.levelOptions[0] != "No info" {
		t.Fatalf("English level options should default to no-info: %#v", en.levelOptions)
	}
	if en.refreshButton != "Refresh" {
		t.Fatalf("unexpected English refresh label: %q", en.refreshButton)
	}
	if ja.refreshButton == en.refreshButton {
		t.Fatalf("language labels should differ")
	}
}
