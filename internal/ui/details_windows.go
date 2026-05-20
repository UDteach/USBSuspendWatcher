package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"usb-suspend-watch/internal/model"
)

func formatDevice(d model.DeviceSnapshot, language displayLanguage, monitored bool, history []model.Event) string {
	text := stringsFor(language)
	monitoring := text.monitorOff
	if monitored {
		monitoring = text.monitorOn
	}
	lines := []string{
		text.deviceDetailsTitle,
		text.deviceMonitoring + ": " + monitoring,
		text.deviceState + ": " + deviceCurrentState(d, monitored, language),
		text.deviceName + ": " + d.DisplayName(),
		"Instance ID: " + d.InstanceID,
		"Hardware ID: " + d.HardwareID,
		"VID/PID: " + d.VIDPID(),
		"Revision: " + d.Revision,
		"Serial: " + d.Serial,
		"COM port: " + d.COMPort,
		"FTDI serial target: " + formatBool(d.LooksLikeFTDISerial()),
		"FTDI related converter: " + formatBool(d.HasFTDISignal() && d.RelationRole == "converter" && len(d.RelatedInstanceIDs) > 0),
		"Relation role: " + d.RelationRole,
		"Logical group: " + d.LogicalGroupID,
		"Logical group reason: " + d.LogicalGroupReason,
		"Related instance IDs: " + strings.Join(d.RelatedInstanceIDs, " | "),
		text.devicePowerState + ": " + string(d.PowerState),
		"Power evidence: " + d.PowerStateEvidence,
		"Power data hex: " + d.PowerDataHex,
		text.deviceManufacturer + ": " + d.Manufacturer,
		"Service: " + d.Service,
		"Class: " + d.Class,
		"Enumerator: " + d.Enumerator,
		text.deviceLocation + ": " + d.Location,
		"Location paths: " + strings.Join(d.LocationPaths, " | "),
		"Physical device object: " + d.PhysicalDeviceObjectName,
		"Parent instance ID: " + d.ParentInstanceID,
		"Parent / hub chain: " + strings.Join(d.ParentChain, " <- "),
		"Parent / hub states: " + formatParentStates(d.ParentStates),
		"Parent low-power while child D0: " + formatBool(d.ParentLowPowerChildD0),
		"Relation / hub tree:",
	}
	lines = append(lines, formatRelationTree(d)...)
	lines = append(lines,
		"Connected at: "+formatOptionalTime(d.ConnectedAt),
		"Last changed: "+formatOptionalTime(d.LastChanged),
		text.deviceLastSeen+": "+d.LastSeen.Format(time.RFC3339),
	)
	if len(history) > 0 {
		lines = append(lines, "", "Recent sequence:")
		for _, event := range history {
			lines = append(lines, formatSequenceLine(event))
		}
	}
	return strings.Join(lines, "\r\n")
}

func formatEvent(e model.Event, language displayLanguage, monitored bool, wakeCorrelation []model.Event) string {
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
		formatDevice(e.Device, language, monitored, nil),
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
	if len(wakeCorrelation) > 0 && (e.Type == model.EventSystemWake || e.Type == model.EventSystemSleep) {
		lines = append(lines, "", "Wake correlation:")
		for _, event := range wakeCorrelation {
			lines = append(lines, formatSequenceLine(event))
		}
	}
	return strings.Join(lines, "\r\n")
}

func formatParentStates(states []model.ParentDeviceState) string {
	if len(states) == 0 {
		return ""
	}
	parts := make([]string, 0, len(states))
	for _, state := range states {
		name := state.DisplayName
		if name == "" {
			name = state.InstanceID
		}
		parts = append(parts, fmt.Sprintf("%s=%s", name, state.PowerState))
	}
	return strings.Join(parts, " | ")
}

func formatRelationTree(d model.DeviceSnapshot) []string {
	var lines []string
	indent := ""
	for i := len(d.ParentStates) - 1; i >= 0; i-- {
		state := d.ParentStates[i]
		name := state.DisplayName
		if name == "" {
			name = state.InstanceID
		}
		lines = append(lines, fmt.Sprintf("%s- parent/hub: %s [%s]", indent, name, state.PowerState))
		indent += "  "
	}
	lines = append(lines, fmt.Sprintf("%s- device: %s [%s, %s]", indent, d.DisplayName(), d.PowerState, d.RelationRole))
	if len(d.RelatedInstanceIDs) > 0 {
		for _, id := range d.RelatedInstanceIDs {
			lines = append(lines, fmt.Sprintf("%s  - related candidate: %s", indent, id))
		}
	}
	if len(lines) == 0 {
		return []string{"  (no relation data)"}
	}
	return lines
}

func formatEventSequence(events []model.Event) string {
	lines := make([]string, 0, len(events))
	for _, event := range events {
		lines = append(lines, formatSequenceLine(event))
	}
	return strings.Join(lines, " || ")
}

func formatSequenceLine(event model.Event) string {
	return fmt.Sprintf(
		"%s  %s  %s  %s  %s",
		event.Time.Format("2006-01-02 15:04:05"),
		event.Type,
		event.Source,
		event.Device.DisplayName(),
		event.Message,
	)
}

func formatBool(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func formatOptionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func versionOrDev(v string) string {
	if v == "" {
		return "dev"
	}
	return v
}
