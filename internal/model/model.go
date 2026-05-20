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
	EventSystemSleep      EventType = "system_sleep"
	EventSystemWake       EventType = "system_wake"
	EventInfo             EventType = "info"
	EventError            EventType = "error"
)

type Source string

const (
	SourceSetupAPIPoll   Source = "setupapi_poll"
	SourceDeviceChange   Source = "wm_devicechange"
	SourceETWUSBHUB3     Source = "etw_usbhub3"
	SourceETWUCX         Source = "etw_ucx"
	SourceETWUSBXHCI     Source = "etw_usbxhci"
	SourcePowerBroadcast Source = "wm_powerbroadcast"
	SourceApp            Source = "app"
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
	InstanceID               string           `json:"instance_id"`
	Description              string           `json:"description,omitempty"`
	FriendlyName             string           `json:"friendly_name,omitempty"`
	Manufacturer             string           `json:"manufacturer,omitempty"`
	Service                  string           `json:"service,omitempty"`
	Class                    string           `json:"class,omitempty"`
	Enumerator               string           `json:"enumerator,omitempty"`
	Location                 string           `json:"location,omitempty"`
	LocationPaths            []string         `json:"location_paths,omitempty"`
	HardwareID               string           `json:"hardware_id,omitempty"`
	COMPort                  string           `json:"com_port,omitempty"`
	PhysicalDeviceObjectName string           `json:"physical_device_object_name,omitempty"`
	ParentInstanceID         string           `json:"parent_instance_id,omitempty"`
	ParentChain              []string         `json:"parent_chain,omitempty"`
	VID                      string           `json:"vid,omitempty"`
	PID                      string           `json:"pid,omitempty"`
	Revision                 string           `json:"revision,omitempty"`
	Serial                   string           `json:"serial,omitempty"`
	PowerState               DevicePowerState `json:"power_state"`
	PowerStateEvidence       string           `json:"power_state_evidence,omitempty"`
	PowerDataHex             string           `json:"power_data_hex,omitempty"`
	Present                  bool             `json:"present"`
	ConnectedAt              time.Time        `json:"connected_at,omitempty"`
	LastChanged              time.Time        `json:"last_changed,omitempty"`
	LastSeen                 time.Time        `json:"last_seen"`
}

func (d DeviceSnapshot) DisplayName() string {
	if d.FriendlyName != "" {
		return withCOMPort(d.FriendlyName, d.COMPort)
	}
	if d.Description != "" {
		return withCOMPort(d.Description, d.COMPort)
	}
	if d.InstanceID != "" {
		return d.InstanceID
	}
	return "(unknown USB device)"
}

func withCOMPort(name, port string) string {
	if port == "" || strings.Contains(strings.ToUpper(name), strings.ToUpper(port)) {
		return name
	}
	return name + " (" + port + ")"
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

func (d DeviceSnapshot) IdentitySummary() string {
	var parts []string
	if vidpid := d.VIDPID(); vidpid != "" {
		parts = append(parts, vidpid)
	}
	if d.Revision != "" {
		parts = append(parts, "REV_"+d.Revision)
	}
	if d.Serial != "" {
		parts = append(parts, "serial="+d.Serial)
	}
	if d.COMPort != "" {
		parts = append(parts, "port="+d.COMPort)
	}
	if d.ParentInstanceID != "" {
		parts = append(parts, "parent="+d.ParentInstanceID)
	}
	if len(parts) == 0 {
		return d.InstanceID
	}
	return strings.Join(parts, " | ")
}

func (d DeviceSnapshot) LooksLikeFTDISerial() bool {
	joined := strings.ToUpper(strings.Join([]string{
		d.VID,
		d.Manufacturer,
		d.FriendlyName,
		d.Description,
		d.Service,
		d.HardwareID,
		d.InstanceID,
	}, " "))
	return d.COMPort != "" && (strings.Contains(joined, "FTDI") || strings.Contains(joined, "VID_0403") || d.VID == "0403")
}

func DeviceEvidenceRaw(d DeviceSnapshot) map[string]string {
	raw := make(map[string]string)
	add := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			raw[key] = value
		}
	}
	add("instance_id", d.InstanceID)
	add("friendly_name", d.FriendlyName)
	add("description", d.Description)
	add("hardware_id", d.HardwareID)
	add("vid", d.VID)
	add("pid", d.PID)
	add("revision", d.Revision)
	add("serial", d.Serial)
	add("com_port", d.COMPort)
	if d.LooksLikeFTDISerial() {
		add("ftdi_serial_target", "true")
	}
	add("power_state", string(d.PowerState))
	add("power_state_evidence", d.PowerStateEvidence)
	add("power_data_hex", d.PowerDataHex)
	add("parent_instance_id", d.ParentInstanceID)
	add("physical_device_object_name", d.PhysicalDeviceObjectName)
	if len(d.LocationPaths) > 0 {
		add("location_paths", strings.Join(d.LocationPaths, " | "))
	}
	if len(d.ParentChain) > 0 {
		add("parent_chain", strings.Join(d.ParentChain, " | "))
	}
	return raw
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
