package ui

import (
	"sort"
	"strings"

	"usb-suspend-watch/internal/model"
)

type eventFilter struct {
	TypeIndex       int
	ConfidenceIndex int
	LevelIndex      int
	Query           string
}

func filterEvents(events []model.Event, filter eventFilter) []model.Event {
	out := make([]model.Event, 0, len(events))
	for _, event := range events {
		if eventMatchesFilter(event, filter) {
			out = append(out, event)
		}
	}
	return out
}

func eventMatchesFilter(event model.Event, filter eventFilter) bool {
	if !eventMatchesType(event, filter.TypeIndex) {
		return false
	}
	if !eventMatchesConfidence(event, filter.ConfidenceIndex) {
		return false
	}
	if !eventMatchesDisplayLevel(event, filter.LevelIndex) {
		return false
	}
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(eventSearchText(event)), query)
}

func eventMatchesType(event model.Event, index int) bool {
	switch index {
	case 1:
		return event.Type == model.EventSuspectSuspend ||
			event.Type == model.EventPowerD0Exit ||
			event.Type == model.EventIdleNotification
	case 2:
		return event.Type == model.EventResume || event.Type == model.EventPowerD0Entry
	case 3:
		return event.Type == model.EventPnPArrival || event.Type == model.EventPnPRemoval
	case 4:
		return event.Type == model.EventError
	default:
		return true
	}
}

func eventMatchesConfidence(event model.Event, index int) bool {
	switch index {
	case 1:
		return event.Confidence == model.ConfidenceHigh || event.Confidence == model.ConfidenceMedium
	case 2:
		return event.Confidence == model.ConfidenceHigh
	default:
		return true
	}
}

func eventMatchesDisplayLevel(event model.Event, index int) bool {
	switch index {
	case 1:
		return eventIsImportant(event)
	case 2:
		return true
	default:
		return event.Type != model.EventInfo
	}
}

func eventIsImportant(event model.Event) bool {
	switch event.Type {
	case model.EventSuspectSuspend,
		model.EventPowerD0Exit,
		model.EventIdleNotification,
		model.EventResume,
		model.EventPowerD0Entry,
		model.EventError:
		return true
	default:
		return false
	}
}

func eventSearchText(event model.Event) string {
	parts := []string{
		string(event.Type),
		string(event.Confidence),
		string(event.Source),
		event.Message,
		event.Provider,
		event.Device.DisplayName(),
		event.Device.InstanceID,
		event.Device.HardwareID,
		event.Device.VIDPID(),
		event.Device.Location,
		event.Device.Manufacturer,
		event.Device.Service,
		event.Device.Class,
		event.Device.Enumerator,
	}
	if len(event.Raw) > 0 {
		keys := make([]string, 0, len(event.Raw))
		for key := range event.Raw {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			parts = append(parts, key, event.Raw[key])
		}
	}
	return strings.Join(parts, " ")
}

func eventMark(event model.Event, language displayLanguage) string {
	text := stringsFor(language)
	switch event.Type {
	case model.EventSuspectSuspend, model.EventPowerD0Exit, model.EventIdleNotification:
		return text.markSuspend
	case model.EventResume, model.EventPowerD0Entry:
		return text.markResume
	case model.EventError:
		return text.markError
	case model.EventPnPArrival:
		return text.markPnPArrival
	case model.EventPnPRemoval:
		return text.markPnPRemoval
	default:
		return ""
	}
}
