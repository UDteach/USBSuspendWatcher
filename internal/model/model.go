package model

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type EventType string

const (
	EventPowerD0Exit      EventType = "power_d0_exit"
	EventPowerD0Entry     EventType = "power_d0_entry"
	EventIdleNotification EventType = "idle_notification"
	EventPnPArrival       EventType = "pnp_arrival"
	EventPnPRemoval       EventType = "pnp_removal"
	EventSuspectSuspend   EventType = "suspect_suspend"
	EventResume           EventType = "resume"
	EventInfo             EventType = "info"
	EventError            EventType = "error"
)

type Source string

const (
	SourceSetupAPIPoll Source = "setupapi_poll"
	SourceDeviceChange Source = "wm_devicechange"
	SourceETWUSBHUB3   Source = "etw_usbhub3"
	SourceETWUCX       Source = "etw_ucx"
	SourceETWUSBXHCI   Source = "etw_usbxhci"
	SourceApp          Source = "app"
)

type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

type DevicePowerState string

const (
	PowerUnknown DevicePowerState = "unknown"
	PowerD0      DevicePowerState = "D0"
	PowerD1      DevicePowerState = "D1"
	PowerD2      DevicePowerState = "D2"
	PowerD3      DevicePowerState = "D3"
)

type DeviceSnapshot struct {
	InstanceID   string           `json:"instance_id"`
	Description  string           `json:"description,omitempty"`
	FriendlyName string           `json:"friendly_name,omitempty"`
	Manufacturer string           `json:"manufacturer,omitempty"`
	Service      string           `json:"service,omitempty"`
	Class        string           `json:"class,omitempty"`
	Enumerator   string           `json:"enumerator,omitempty"`
	Location     string           `json:"location,omitempty"`
	HardwareID   string           `json:"hardware_id,omitempty"`
	VID          string           `json:"vid,omitempty"`
	PID          string           `json:"pid,omitempty"`
	Revision     string           `json:"revision,omitempty"`
	Serial       string           `json:"serial,omitempty"`
	PowerState   DevicePowerState `json:"power_state"`
	Present      bool             `json:"present"`
	LastSeen     time.Time        `json:"last_seen"`
}

func (d DeviceSnapshot) DisplayName() string {
	if d.FriendlyName != "" {
		return d.FriendlyName
	}
	if d.Description != "" {
		return d.Description
	}
	if d.InstanceID != "" {
		return d.InstanceID
	}
	return "(unknown USB device)"
}

func (d DeviceSnapshot) VIDPID() string {
	switch {
	case d.VID != "" && d.PID != "":
		return fmt.Sprintf("VID_%s PID_%s", d.VID, d.PID)
	case d.VID != "":
		return "VID_" + d.VID
	case d.PID != "":
		return "PID_" + d.PID
	default:
		return ""
	}
}

type Event struct {
	Time       time.Time         `json:"time"`
	Type       EventType         `json:"type"`
	Source     Source            `json:"source"`
	Confidence Confidence        `json:"confidence"`
	Device     DeviceSnapshot    `json:"device"`
	Message    string            `json:"message,omitempty"`
	Provider   string            `json:"provider,omitempty"`
	EventID    uint16            `json:"event_id,omitempty"`
	Raw        map[string]string `json:"raw,omitempty"`
}

type ETWRecord struct {
	Time       time.Time
	Provider   string
	EventID    uint16
	Task       string
	Opcode     string
	Properties map[string]string
}

var (
	vidRe = regexp.MustCompile(`(?i)\bVID[_-]?([0-9A-F]{4})\b`)
	pidRe = regexp.MustCompile(`(?i)\bPID[_-]?([0-9A-F]{4})\b`)
	revRe = regexp.MustCompile(`(?i)\bREV[_-]?([0-9A-F]{4})\b`)
)

func PopulateUSBIDs(d *DeviceSnapshot) {
	raw := d.InstanceID + " " + d.HardwareID
	if d.VID == "" {
		d.VID = firstMatch(vidRe, raw)
	}
	if d.PID == "" {
		d.PID = firstMatch(pidRe, raw)
	}
	if d.Revision == "" {
		d.Revision = firstMatch(revRe, raw)
	}
	if d.Serial == "" {
		parts := strings.Split(d.InstanceID, `\`)
		if len(parts) > 2 {
			d.Serial = parts[len(parts)-1]
		}
	}
	d.VID = strings.ToUpper(d.VID)
	d.PID = strings.ToUpper(d.PID)
	d.Revision = strings.ToUpper(d.Revision)
}

func firstMatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func IsLowPowerState(s DevicePowerState) bool {
	return s == PowerD1 || s == PowerD2 || s == PowerD3
}
