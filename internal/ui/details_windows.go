package ui

import (
	"encoding/json"
	"fmt"
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
		"Diagnostic summary: " + strings.Join(diagnosticSummary(d, history), " | "),
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
		"Same-device candidate: " + formatDiagnosticScore(d),
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
	if len(wakeCorrelation) > 0 && (e.Type == model.EventSystemWake || e.Type == model.EventSystemSleep) {
		lines = append(lines, "", "Wake correlation:")
		for _, event := range wakeCorrelation {
			lines = append(lines, formatSequenceLine(event))
		}
	}
	return strings.Join(lines, "\r\n")
}

func formatDeviceRaw(d model.DeviceSnapshot, history []model.Event) string {
	raw := model.DeviceEvidenceRaw(d)
	if summary := strings.Join(diagnosticSummary(d, history), " | "); summary != "" {
		raw["diagnostic_summary"] = summary
	}
	return formatPrettyJSON(raw)
}

func formatEventRaw(e model.Event) string {
	return formatPrettyJSON(e)
}

func formatPrettyJSON(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("failed to format raw JSON: %v", err)
	}
	return string(data)
}

func diagnosticSummary(d model.DeviceSnapshot, history []model.Event) []string {
	var lines []string
	name := d.COMPort
	if name == "" {
		name = d.DisplayName()
	}
	if !d.ConnectedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("%s connected at %s", name, d.ConnectedAt.Format("15:04:05")))
	}
	if d.PowerState != "" && d.PowerState != model.PowerUnknown {
		lines = append(lines, fmt.Sprintf("%s reports %s from %s", name, d.PowerState, emptyAsUnknown(d.PowerStateEvidence)))
	}
	if d.DiagnosticScore > 0 {
		lines = append(lines, fmt.Sprintf("same-device candidate %d%%: %s", d.DiagnosticScore, strings.Join(d.DiagnosticReasons, ", ")))
	} else if d.LogicalGroupReason != "" {
		lines = append(lines, "same-device candidate has no evidence: "+d.LogicalGroupReason)
	}
	if d.ParentLowPowerChildD0 {
		lines = append(lines, "parent hub/device was low-power while child reported D0")
	} else if hasUnknownParentState(d.ParentStates) {
		lines = append(lines, "parent state includes unknown entries")
	}
	if wake := latestWakeSummary(history); wake != "" {
		lines = append(lines, wake)
	}
	if len(lines) == 0 {
		lines = append(lines, "no session transition evidence selected")
	}
	return lines
}

func formatDiagnosticScore(d model.DeviceSnapshot) string {
	reasons := strings.Join(d.DiagnosticReasons, " | ")
	if d.DiagnosticScore <= 0 {
		if reasons == "" {
			reasons = "no same-device evidence"
		}
		return "0% | " + reasons
	}
	return fmt.Sprintf("%d%% | %s", d.DiagnosticScore, reasons)
}

func emptyAsUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown evidence"
	}
	return value
}

func hasUnknownParentState(states []model.ParentDeviceState) bool {
	for _, state := range states {
		if state.PowerState == model.PowerUnknown {
			return true
		}
	}
	return false
}

func latestWakeSummary(history []model.Event) string {
	for i := len(history) - 1; i >= 0; i-- {
		event := history[i]
		if event.Type != model.EventSystemWake {
			continue
		}
		confidence := event.Raw["wake_confidence"]
		if confidence == "" {
			confidence = "unknown"
		}
		reasons := event.Raw["wake_reasons"]
		if reasons != "" {
			return "wake source confidence " + confidence + ": " + reasons
		}
		return "wake source confidence " + confidence
	}
	return ""
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
