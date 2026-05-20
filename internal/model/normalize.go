package model

import (
	"fmt"
	"strings"
	"time"
)

func NormalizePowerTransition(previous, current DeviceSnapshot) []Event {
	now := current.LastSeen
	if now.IsZero() {
		now = time.Now()
	}
	base := Event{
		Time:       now,
		Source:     SourceSetupAPIPoll,
		Confidence: ConfidenceMedium,
		Device:     current,
		Raw: map[string]string{
			"previous_power_state": string(previous.PowerState),
			"current_power_state":  string(current.PowerState),
		},
	}
	for key, value := range DeviceEvidenceRaw(current) {
		base.Raw[key] = value
	}
	if previous.PowerStateEvidence != "" {
		base.Raw["previous_power_state_evidence"] = previous.PowerStateEvidence
	}
	if previous.PowerDataHex != "" {
		base.Raw["previous_power_data_hex"] = previous.PowerDataHex
	}

	if previous.PowerState == current.PowerState || current.PowerState == PowerUnknown {
		return nil
	}

	switch {
	case previous.PowerState == PowerD0 && IsLowPowerState(current.PowerState):
		out := base
		out.Type = EventPowerD0Exit
		out.Message = fmt.Sprintf("device power state changed from %s to %s", previous.PowerState, current.PowerState)
		suspect := out
		suspect.Type = EventSuspectSuspend
		suspect.Confidence = ConfidenceMedium
		suspect.Message = "device left D0 and is now in a low-power state; treating as suspected selective suspend"
		return []Event{out, suspect}
	case IsLowPowerState(previous.PowerState) && current.PowerState == PowerD0:
		out := base
		out.Type = EventPowerD0Entry
		out.Message = fmt.Sprintf("device power state changed from %s to %s", previous.PowerState, current.PowerState)
		if current.ParentLowPowerChildD0 {
			out.Message += "; parent hub/device is low power while child reports D0"
		}
		resume := out
		resume.Type = EventResume
		resume.Confidence = ConfidenceMedium
		resume.Message = "device returned to D0 after a low-power state"
		if current.ParentLowPowerChildD0 {
			resume.Message += "; parent hub/device is low power while child reports D0"
		}
		return []Event{out, resume}
	default:
		out := base
		out.Type = EventInfo
		out.Confidence = ConfidenceLow
		out.Message = fmt.Sprintf("device power state changed from %s to %s", previous.PowerState, current.PowerState)
		return []Event{out}
	}
}

func NormalizeETW(record ETWRecord) Event {
	t := record.Time
	if t.IsZero() {
		t = time.Now()
	}
	device := deviceFromETW(record.Properties)
	source := sourceForProvider(record.Provider)
	eventType, confidence, message := classifyETW(record)

	return Event{
		Time:       t,
		Type:       eventType,
		Source:     source,
		Confidence: confidence,
		Device:     device,
		Message:    message,
		Provider:   record.Provider,
		EventID:    record.EventID,
		Raw:        record.Properties,
	}
}

func classifyETW(record ETWRecord) (EventType, Confidence, string) {
	provider := strings.ToLower(record.Provider)
	switch {
	case strings.Contains(provider, "usbhub3"):
		switch record.EventID {
		case 51:
			return EventPowerD0Entry, ConfidenceMedium, "USBHUB3 started EvtDeviceD0Entry for a USB device"
		case 52:
			return EventPowerD0Entry, ConfidenceHigh, "USBHUB3 completed EvtDeviceD0Entry for a USB device"
		case 53:
			return EventPowerD0Exit, ConfidenceMedium, "USBHUB3 started EvtDeviceD0Exit for a USB device"
		case 54:
			return EventPowerD0Exit, ConfidenceHigh, "USBHUB3 completed EvtDeviceD0Exit for a USB device"
		case 96, 97, 98:
			return EventIdleNotification, ConfidenceHigh, "USB idle notification event"
		case 194, 195, 196:
			return EventSuspectSuspend, ConfidenceMedium, "USBHUB3 idle-power diagnostic event"
		}
	case strings.Contains(provider, "usbxhci"):
		if strings.Contains(strings.ToLower(record.Task+" "+record.Opcode), "d0") {
			return EventPowerD0Exit, ConfidenceMedium, "USBXHCI controller D0 transition event"
		}
	case strings.Contains(provider, "usb-ucx") || strings.Contains(provider, "ucx"):
		if strings.Contains(strings.ToLower(record.Task+" "+record.Opcode), "idle") {
			return EventIdleNotification, ConfidenceMedium, "UCX idle/power event"
		}
	}

	taskOp := strings.TrimSpace(record.Task + " " + record.Opcode)
	if taskOp == "" {
		taskOp = fmt.Sprintf("ETW event %d", record.EventID)
	}
	return EventInfo, ConfidenceLow, taskOp
}

func sourceForProvider(provider string) Source {
	p := strings.ToLower(provider)
	switch {
	case strings.Contains(p, "usbhub3"):
		return SourceETWUSBHUB3
	case strings.Contains(p, "usbxhci"):
		return SourceETWUSBXHCI
	case strings.Contains(p, "usb-ucx") || strings.Contains(p, "ucx"):
		return SourceETWUCX
	default:
		return SourceApp
	}
}

func deviceFromETW(properties map[string]string) DeviceSnapshot {
	var d DeviceSnapshot
	for key, value := range properties {
		lk := strings.ToLower(key)
		switch {
		case d.InstanceID == "" && (strings.Contains(lk, "instance") || strings.Contains(lk, "deviceid")):
			d.InstanceID = value
		case d.FriendlyName == "" && strings.Contains(lk, "name"):
			d.FriendlyName = value
		case d.Location == "" && strings.Contains(lk, "port"):
			d.Location = value
		}
		if d.InstanceID == "" && (strings.Contains(strings.ToUpper(value), "VID_") || strings.Contains(strings.ToUpper(value), "PID_")) {
			d.InstanceID = value
		}
	}
	if d.HardwareID == "" {
		d.HardwareID = d.InstanceID
	}
	PopulateUSBIDs(&d)
	d.Present = true
	d.LastSeen = time.Now()
	if d.PowerState == "" {
		d.PowerState = PowerUnknown
	}
	return d
}
