package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"usb-suspend-watch/internal/model"
)

func formatDevice(d model.DeviceSnapshot, language displayLanguage) string {
	text := stringsFor(language)
	lines := []string{
		text.deviceDetailsTitle,
		text.deviceName + ": " + d.DisplayName(),
		"Instance ID: " + d.InstanceID,
		"Hardware ID: " + d.HardwareID,
		"VID/PID: " + d.VIDPID(),
		"Revision: " + d.Revision,
		"Serial: " + d.Serial,
		text.devicePowerState + ": " + string(d.PowerState),
		text.deviceManufacturer + ": " + d.Manufacturer,
		"Service: " + d.Service,
		"Class: " + d.Class,
		"Enumerator: " + d.Enumerator,
		text.deviceLocation + ": " + d.Location,
		text.deviceLastSeen + ": " + d.LastSeen.Format(time.RFC3339),
	}
	return strings.Join(lines, "\r\n")
}

func formatEvent(e model.Event, language displayLanguage) string {
	text := stringsFor(language)
	lines := []string{
		text.eventDetailsTitle,
		text.eventMark + ": " + eventMark(e, language),
		text.eventTime + ": " + e.Time.Format(time.RFC3339Nano),
		text.eventType + ": " + string(e.Type),
		"Source: " + string(e.Source),
		text.eventConfidence + ": " + string(e.Confidence),
		text.eventMessage + ": " + e.Message,
		"Provider: " + e.Provider,
		fmt.Sprintf("Event ID: %d", e.EventID),
		"",
		formatDevice(e.Device, language),
	}
	if len(e.Raw) > 0 {
		lines = append(lines, "", text.rawETWProperties+":")
		keys := make([]string, 0, len(e.Raw))
		for key := range e.Raw {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, key+": "+e.Raw[key])
		}
	}
	return strings.Join(lines, "\r\n")
}

func versionOrDev(v string) string {
	if v == "" {
		return "dev"
	}
	return v
}
