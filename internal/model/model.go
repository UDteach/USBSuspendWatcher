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
	EventDStateTransition EventType = "dstate_transition"
	EventParentMismatch   EventType = "parent_dstate_mismatch"
	EventProblemCode      EventType = "problem_code"
	EventStatusChanged    EventType = "status_changed"
	EventDeviceMissing    EventType = "device_missing"
	EventDeviceReenum     EventType = "device_reenumerated"
	EventLastSeenStale    EventType = "last_seen_stale"
	EventNetworkStats     EventType = "network_stats"
	EventDetailedLogging  EventType = "detailed_logging_started"
	EventInfo             EventType = "info"
	EventError            EventType = "error"
)

type Source string

const (
	SourceSetupAPIPoll     Source = "setupapi_poll"
	SourceDeviceChange     Source = "wm_devicechange"
	SourceETWUSBHUB3       Source = "etw_usbhub3"
	SourceETWUCX           Source = "etw_ucx"
	SourceETWUSBXHCI       Source = "etw_usbxhci"
	SourcePowerBroadcast   Source = "wm_powerbroadcast"
	SourcePowercfgLastWake Source = "powercfg_lastwake"
	SourceUSBPcap          Source = "usbpcap"
	SourceApp              Source = "app"
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
	InstanceID               string              `json:"instance_id"`
	Description              string              `json:"description,omitempty"`
	FriendlyName             string              `json:"friendly_name,omitempty"`
	Manufacturer             string              `json:"manufacturer,omitempty"`
	Service                  string              `json:"service,omitempty"`
	Class                    string              `json:"class,omitempty"`
	ClassGuid                string              `json:"class_guid,omitempty"`
	Driver                   string              `json:"driver,omitempty"`
	ContainerID              string              `json:"container_id,omitempty"`
	BusReportedDeviceDesc    string              `json:"bus_reported_device_desc,omitempty"`
	Enumerator               string              `json:"enumerator,omitempty"`
	Location                 string              `json:"location,omitempty"`
	LocationPaths            []string            `json:"location_paths,omitempty"`
	HardwareID               string              `json:"hardware_id,omitempty"`
	COMPort                  string              `json:"com_port,omitempty"`
	PhysicalDeviceObjectName string              `json:"physical_device_object_name,omitempty"`
	ParentInstanceID         string              `json:"parent_instance_id,omitempty"`
	ParentChain              []string            `json:"parent_chain,omitempty"`
	ParentStates             []ParentDeviceState `json:"parent_states,omitempty"`
	ParentLowPowerChildD0    bool                `json:"parent_low_power_child_d0,omitempty"`
	LogicalGroupID           string              `json:"logical_group_id,omitempty"`
	LogicalGroupReason       string              `json:"logical_group_reason,omitempty"`
	DiagnosticScore          int                 `json:"diagnostic_score,omitempty"`
	DiagnosticReasons        []string            `json:"diagnostic_reasons,omitempty"`
	GroupDisplayName         string              `json:"group_display_name,omitempty"`
	SessionObserved          bool                `json:"session_observed,omitempty"`
	RelationRole             string              `json:"relation_role,omitempty"`
	RelatedInstanceIDs       []string            `json:"related_instance_ids,omitempty"`
	VID                      string              `json:"vid,omitempty"`
	PID                      string              `json:"pid,omitempty"`
	Revision                 string              `json:"revision,omitempty"`
	Serial                   string              `json:"serial,omitempty"`
	ConfigManagerErrorCode   uint32              `json:"config_manager_error_code,omitempty"`
	ProblemCode              uint32              `json:"problem_code,omitempty"`
	StatusFlags              uint32              `json:"status_flags,omitempty"`
	StatusFlagNames          []string            `json:"status_flag_names,omitempty"`
	Status                   string              `json:"status,omitempty"`
	PowerState               DevicePowerState    `json:"power_state"`
	PowerData                PowerData           `json:"power_data,omitempty"`
	PowerStateEvidence       string              `json:"power_state_evidence,omitempty"`
	PowerDataHex             string              `json:"power_data_hex,omitempty"`
	USBPcapHints             USBPcapHints        `json:"usbpcap_hints,omitempty"`
	CorrelationID            string              `json:"correlation_id,omitempty"`
	Present                  bool                `json:"present"`
	ConnectedAt              time.Time           `json:"connected_at,omitempty"`
	LastChanged              time.Time           `json:"last_changed,omitempty"`
	LastSeen                 time.Time           `json:"last_seen"`
}

type ParentDeviceState struct {
	InstanceID    string           `json:"instance_id"`
	DisplayName   string           `json:"display_name,omitempty"`
	Service       string           `json:"service,omitempty"`
	Class         string           `json:"class,omitempty"`
	ClassGuid     string           `json:"class_guid,omitempty"`
	Driver        string           `json:"driver,omitempty"`
	Enumerator    string           `json:"enumerator,omitempty"`
	Location      string           `json:"location,omitempty"`
	LocationPaths []string         `json:"location_paths,omitempty"`
	ContainerID   string           `json:"container_id,omitempty"`
	PowerState    DevicePowerState `json:"power_state"`
	Evidence      string           `json:"evidence,omitempty"`
}

type PowerData struct {
	Size                    uint32             `json:"pd_size,omitempty"`
	MostRecentPowerState    DevicePowerState   `json:"pd_most_recent_power_state,omitempty"`
	MostRecentPowerStateRaw uint32             `json:"pd_most_recent_power_state_raw,omitempty"`
	Capabilities            uint32             `json:"pd_capabilities,omitempty"`
	D1Latency               uint32             `json:"pd_d1_latency,omitempty"`
	D2Latency               uint32             `json:"pd_d2_latency,omitempty"`
	D3Latency               uint32             `json:"pd_d3_latency,omitempty"`
	PowerStateMapping       []DevicePowerState `json:"pd_power_state_mapping,omitempty"`
	PowerStateMappingRaw    []uint32           `json:"pd_power_state_mapping_raw,omitempty"`
	DeepestSystemWake       string             `json:"pd_deepest_system_wake,omitempty"`
	DeepestSystemWakeRaw    uint32             `json:"pd_deepest_system_wake_raw,omitempty"`
	D3HotColdNote           string             `json:"d3_hot_cold_note,omitempty"`
}

type USBPcapHints struct {
	RootHub            string `json:"root_hub,omitempty"`
	Interface          string `json:"interface,omitempty"`
	BusNumber          string `json:"bus_number,omitempty"`
	DeviceAddress      string `json:"device_address,omitempty"`
	BulkInEndpoint     string `json:"bulk_in_endpoint,omitempty"`
	BulkOutEndpoint    string `json:"bulk_out_endpoint,omitempty"`
	EndpointConfidence string `json:"endpoint_confidence,omitempty"`
}

type NetAdapterSnapshot struct {
	Time             time.Time `json:"time"`
	CorrelationID    string    `json:"correlation_id,omitempty"`
	Name             string    `json:"name,omitempty"`
	InterfaceIndex   string    `json:"interface_index,omitempty"`
	Status           string    `json:"status,omitempty"`
	LinkSpeed        string    `json:"link_speed,omitempty"`
	OutboundErrors   string    `json:"outbound_errors,omitempty"`
	InboundErrors    string    `json:"inbound_errors,omitempty"`
	DiscardedPackets string    `json:"discarded_packets,omitempty"`
	DeviceInstanceID string    `json:"device_instance_id,omitempty"`
}

type DiagnosticCause struct {
	Kind       string     `json:"kind"`
	Reason     string     `json:"reason"`
	Confidence Confidence `json:"confidence"`
	Evidence   []string   `json:"evidence,omitempty"`
}

func DiagnosticCauses(d DeviceSnapshot, history []Event) []DiagnosticCause {
	var out []DiagnosticCause
	if d.ParentLowPowerChildD0 || hasLowPowerParent(d.ParentStates) {
		out = append(out, DiagnosticCause{
			Kind:       "USB電源管理疑い",
			Reason:     "parent Hub/xHCI/USB4 Router low-power state is present near the selected device",
			Confidence: ConfidenceMedium,
			Evidence:   []string{formatParentStateEvidence(d.ParentStates), d.PowerStateEvidence},
		})
	}
	if looksNetworkRelated(d) && !hasLowPowerParent(d.ParentStates) && d.ProblemCode == 0 {
		out = append(out, DiagnosticCause{
			Kind:       "NDIS / ドライバ疑い",
			Reason:     "USB chain has no current low-power/problem evidence; compare network statistics and USBPcap Bulk OUT",
			Confidence: ConfidenceLow,
			Evidence:   []string{"class=" + d.Class, "service=" + d.Service},
		})
	}
	for _, event := range history {
		if event.Raw == nil {
			continue
		}
		bulkOut := strings.ToLower(event.Raw["usbpcap_bulk_out"])
		complete := strings.ToLower(event.Raw["usbpcap_complete"])
		status := strings.ToLower(event.Raw["usbpcap_status"])
		if bulkOut == "true" && (complete == "false" || strings.Contains(status, "error")) {
			out = append(out, DiagnosticCause{
				Kind:       "USB転送詰まり疑い",
				Reason:     "USBPcap evidence says Bulk OUT exists but completion/error evidence is suspicious",
				Confidence: ConfidenceMedium,
				Evidence:   []string{event.Message, "usbpcap_status=" + event.Raw["usbpcap_status"]},
			})
			break
		}
		if bulkOut == "success" && strings.EqualFold(event.Raw["upper_ack_observed"], "false") {
			out = append(out, DiagnosticCause{
				Kind:       "ドングルFW / PHY疑い",
				Reason:     "Bulk OUT succeeded but no upper response/ACK evidence is present",
				Confidence: ConfidenceLow,
				Evidence:   []string{event.Message},
			})
			break
		}
	}
	if len(out) == 0 {
		out = append(out, DiagnosticCause{
			Kind:       "未判定",
			Reason:     "current evidence is insufficient; use D-state history, ETW, USBPcap, and network logs",
			Confidence: ConfidenceLow,
		})
	}
	return out
}

func hasLowPowerParent(states []ParentDeviceState) bool {
	for _, state := range states {
		if IsLowPowerState(state.PowerState) {
			return true
		}
	}
	return false
}

func formatParentStateEvidence(states []ParentDeviceState) string {
	if len(states) == 0 {
		return "parent_states=none"
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

func looksNetworkRelated(d DeviceSnapshot) bool {
	text := strings.ToLower(strings.Join([]string{d.Class, d.Service, d.Description, d.FriendlyName, d.BusReportedDeviceDesc}, " "))
	return strings.Contains(text, "net") || strings.Contains(text, "ndis") || strings.Contains(text, "rndis") || strings.Contains(text, "ethernet")
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
	return d.COMPort != "" && d.HasFTDISignal()
}

func (d DeviceSnapshot) HasFTDISignal() bool {
	joined := strings.ToUpper(strings.Join([]string{
		d.VID,
		d.Manufacturer,
		d.FriendlyName,
		d.Description,
		d.Service,
		d.HardwareID,
		d.InstanceID,
	}, " "))
	return strings.Contains(joined, "FTDI") ||
		strings.Contains(joined, "VID_0403") ||
		strings.Contains(joined, "USB SERIAL PORT") ||
		strings.Contains(joined, "FTDIBUS") ||
		d.VID == "0403"
}

func EnrichDeviceRelationships(devices []DeviceSnapshot, allDevices ...[]DeviceSnapshot) []DeviceSnapshot {
	out := make([]DeviceSnapshot, len(devices))
	copy(out, devices)

	byInstance := make(map[string]DeviceSnapshot, len(out))
	for _, all := range allDevices {
		for _, device := range all {
			if device.InstanceID != "" {
				byInstance[strings.ToUpper(device.InstanceID)] = device
			}
		}
	}
	for _, device := range out {
		if device.InstanceID != "" {
			byInstance[strings.ToUpper(device.InstanceID)] = device
		}
	}

	groups := make(map[string][]int)
	for i := range out {
		out[i].RelationRole = classifyRelationRole(out[i])
		out[i].LogicalGroupID, out[i].LogicalGroupReason = logicalGroupKey(out[i])
		if out[i].LogicalGroupID != "" {
			groups[out[i].LogicalGroupID] = append(groups[out[i].LogicalGroupID], i)
		}
		out[i].ParentStates = parentStatesFor(out[i], byInstance)
		out[i].ParentLowPowerChildD0 = parentLowPowerWhileChildD0(out[i])
		out[i].DiagnosticScore, out[i].DiagnosticReasons = diagnosticScore(out[i])
		out[i].GroupDisplayName = groupDisplayName(out[i])
	}

	for _, indexes := range groups {
		if len(indexes) < 2 {
			continue
		}
		for _, idx := range indexes {
			related := make([]string, 0, len(indexes)-1)
			for _, other := range indexes {
				if other == idx {
					continue
				}
				if out[other].InstanceID != "" {
					related = append(related, out[other].InstanceID)
				}
			}
			out[idx].RelatedInstanceIDs = related
		}
	}
	return out
}

func diagnosticScore(d DeviceSnapshot) (int, []string) {
	var reasons []string
	score := 0
	switch d.LogicalGroupReason {
	case "VID/PID + serial":
		score = 90
		reasons = append(reasons, "serial match candidate")
	case "VID/PID + parent instance":
		score = 70
		reasons = append(reasons, "parent instance match candidate")
	case "VID/PID + location paths":
		score = 60
		reasons = append(reasons, "location path match candidate")
	case "VID/PID only is not enough":
		reasons = append(reasons, "VID/PID only is not enough")
	case "missing VID/PID":
		reasons = append(reasons, "missing VID/PID")
	default:
		if strings.TrimSpace(d.LogicalGroupReason) != "" {
			reasons = append(reasons, d.LogicalGroupReason)
		}
	}
	if d.ParentLowPowerChildD0 {
		reasons = append(reasons, "parent low-power while child reports D0")
	}
	if d.RelationRole != "" {
		reasons = append(reasons, "role="+d.RelationRole)
	}
	return score, reasons
}

func groupDisplayName(d DeviceSnapshot) string {
	prefix := "USB Adapter"
	if d.HasFTDISignal() {
		prefix = "FTDI Adapter"
	}
	switch {
	case d.Serial != "":
		return prefix + " " + d.Serial
	case d.COMPort != "":
		return prefix + " " + d.COMPort
	case d.LogicalGroupID != "":
		return prefix + " " + d.VIDPID()
	case d.DisplayName() != "":
		return d.DisplayName()
	default:
		return prefix
	}
}

func classifyRelationRole(d DeviceSnapshot) string {
	joined := strings.ToLower(strings.Join([]string{
		d.FriendlyName,
		d.Description,
		d.Service,
		d.Class,
		d.HardwareID,
		d.InstanceID,
	}, " "))
	switch {
	case d.COMPort != "" || strings.Contains(joined, "usb serial port") || strings.EqualFold(d.Class, "Ports"):
		return "port"
	case strings.Contains(joined, "converter") || strings.Contains(joined, "ftdibus"):
		return "converter"
	default:
		return "device"
	}
}

func logicalGroupKey(d DeviceSnapshot) (string, string) {
	if d.VID == "" || d.PID == "" {
		return "", "missing VID/PID"
	}
	prefix := strings.ToUpper(d.VID) + ":" + strings.ToUpper(d.PID)
	if d.Serial != "" {
		return "vidpid-serial:" + prefix + ":" + strings.ToUpper(d.Serial), "VID/PID + serial"
	}
	if d.ParentInstanceID != "" {
		return "vidpid-parent:" + prefix + ":" + strings.ToUpper(d.ParentInstanceID), "VID/PID + parent instance"
	}
	if len(d.LocationPaths) > 0 {
		return "vidpid-location:" + prefix + ":" + strings.ToUpper(strings.Join(d.LocationPaths, "|")), "VID/PID + location paths"
	}
	return "", "VID/PID only is not enough"
}

func parentStatesFor(d DeviceSnapshot, byInstance map[string]DeviceSnapshot) []ParentDeviceState {
	if len(d.ParentChain) == 0 {
		return nil
	}
	out := make([]ParentDeviceState, 0, len(d.ParentChain))
	for _, id := range d.ParentChain {
		parent, ok := byInstance[strings.ToUpper(id)]
		if !ok {
			out = append(out, ParentDeviceState{
				InstanceID: id,
				PowerState: PowerUnknown,
				Evidence:   "parent not found in all-present-devices snapshot",
			})
			continue
		}
		out = append(out, ParentDeviceState{
			InstanceID:    parent.InstanceID,
			DisplayName:   parent.DisplayName(),
			Service:       parent.Service,
			Class:         parent.Class,
			ClassGuid:     parent.ClassGuid,
			Driver:        parent.Driver,
			Enumerator:    parent.Enumerator,
			Location:      parent.Location,
			LocationPaths: append([]string(nil), parent.LocationPaths...),
			ContainerID:   parent.ContainerID,
			PowerState:    parent.PowerState,
			Evidence:      parent.PowerStateEvidence,
		})
	}
	return out
}

func parentLowPowerWhileChildD0(d DeviceSnapshot) bool {
	if d.PowerState != PowerD0 {
		return false
	}
	for _, parent := range d.ParentStates {
		if IsLowPowerState(parent.PowerState) {
			return true
		}
	}
	return false
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
	add("container_id", d.ContainerID)
	add("class_guid", d.ClassGuid)
	add("driver", d.Driver)
	add("bus_reported_device_desc", d.BusReportedDeviceDesc)
	add("vid", d.VID)
	add("pid", d.PID)
	add("revision", d.Revision)
	add("serial", d.Serial)
	add("com_port", d.COMPort)
	if d.LooksLikeFTDISerial() {
		add("ftdi_serial_target", "true")
	} else if d.HasFTDISignal() && d.RelationRole == "converter" && len(d.RelatedInstanceIDs) > 0 {
		add("ftdi_related_converter", "true")
	}
	add("logical_group_id", d.LogicalGroupID)
	add("logical_group_reason", d.LogicalGroupReason)
	if d.DiagnosticScore > 0 || len(d.DiagnosticReasons) > 0 {
		add("diagnostic_score", fmt.Sprintf("%d", d.DiagnosticScore))
	}
	if len(d.DiagnosticReasons) > 0 {
		add("diagnostic_reasons", strings.Join(d.DiagnosticReasons, " | "))
	}
	add("group_display_name", d.GroupDisplayName)
	if d.SessionObserved {
		add("session_observed", "true")
	}
	add("relation_role", d.RelationRole)
	if len(d.RelatedInstanceIDs) > 0 {
		add("related_instance_ids", strings.Join(d.RelatedInstanceIDs, " | "))
	}
	add("power_state", string(d.PowerState))
	add("power_state_evidence", d.PowerStateEvidence)
	add("power_data_hex", d.PowerDataHex)
	if d.PowerData.Size > 0 || d.PowerData.MostRecentPowerState != "" {
		add("pd_size", fmt.Sprintf("%d", d.PowerData.Size))
		add("pd_most_recent_power_state", string(d.PowerData.MostRecentPowerState))
		add("pd_most_recent_power_state_raw", fmt.Sprintf("%d", d.PowerData.MostRecentPowerStateRaw))
		add("pd_capabilities", fmt.Sprintf("0x%08X", d.PowerData.Capabilities))
		add("pd_d1_latency", fmt.Sprintf("%d", d.PowerData.D1Latency))
		add("pd_d2_latency", fmt.Sprintf("%d", d.PowerData.D2Latency))
		add("pd_d3_latency", fmt.Sprintf("%d", d.PowerData.D3Latency))
		add("pd_power_state_mapping", formatPowerStateMapping(d.PowerData))
		add("pd_deepest_system_wake", d.PowerData.DeepestSystemWake)
		add("pd_deepest_system_wake_raw", fmt.Sprintf("%d", d.PowerData.DeepestSystemWakeRaw))
		add("d3_hot_cold_note", d.PowerData.D3HotColdNote)
	}
	add("status", d.Status)
	if d.StatusFlags != 0 {
		add("status_flags", fmt.Sprintf("0x%08X", d.StatusFlags))
	}
	if len(d.StatusFlagNames) > 0 {
		add("status_flag_names", strings.Join(d.StatusFlagNames, " | "))
	}
	if d.ProblemCode != 0 {
		add("problem_code", fmt.Sprintf("%d", d.ProblemCode))
	}
	if d.ConfigManagerErrorCode != 0 {
		add("config_manager_error_code", fmt.Sprintf("%d", d.ConfigManagerErrorCode))
	}
	add("correlation_id", d.CorrelationID)
	if d.USBPcapHints.RootHub != "" || d.USBPcapHints.DeviceAddress != "" || d.USBPcapHints.EndpointConfidence != "" {
		add("usbpcap_root_hub", d.USBPcapHints.RootHub)
		add("usbpcap_interface", d.USBPcapHints.Interface)
		add("usbpcap_bus_number", d.USBPcapHints.BusNumber)
		add("usbpcap_device_address", d.USBPcapHints.DeviceAddress)
		add("usbpcap_bulk_in_endpoint", d.USBPcapHints.BulkInEndpoint)
		add("usbpcap_bulk_out_endpoint", d.USBPcapHints.BulkOutEndpoint)
		add("usbpcap_endpoint_confidence", d.USBPcapHints.EndpointConfidence)
	}
	if d.ParentLowPowerChildD0 {
		add("parent_low_power_child_d0", "true")
	}
	add("parent_instance_id", d.ParentInstanceID)
	if hints := TopologyHints(d); len(hints) > 0 {
		add("topology_hints", strings.Join(hints, " | "))
	}
	add("physical_device_object_name", d.PhysicalDeviceObjectName)
	if len(d.LocationPaths) > 0 {
		add("location_paths", strings.Join(d.LocationPaths, " | "))
	}
	if len(d.ParentChain) > 0 {
		add("parent_chain", strings.Join(d.ParentChain, " | "))
	}
	if len(d.ParentStates) > 0 {
		parentStates := make([]string, 0, len(d.ParentStates))
		for _, parent := range d.ParentStates {
			state := parent.InstanceID + "=" + string(parent.PowerState)
			if parent.Service != "" {
				state += " service=" + parent.Service
			}
			if parent.Driver != "" {
				state += " driver=" + parent.Driver
			}
			parentStates = append(parentStates, state)
		}
		add("parent_states", strings.Join(parentStates, " | "))
	}
	return raw
}

func formatPowerStateMapping(power PowerData) string {
	if len(power.PowerStateMapping) == 0 && len(power.PowerStateMappingRaw) == 0 {
		return ""
	}
	parts := make([]string, 0, len(power.PowerStateMappingRaw))
	for i, raw := range power.PowerStateMappingRaw {
		state := ""
		if i < len(power.PowerStateMapping) {
			state = string(power.PowerStateMapping[i])
		}
		if state == "" {
			state = "unknown"
		}
		parts = append(parts, fmt.Sprintf("S%d=%s(%d)", i, state, raw))
	}
	return strings.Join(parts, " | ")
}

func TopologyHints(d DeviceSnapshot) []string {
	var hints []string
	for _, parent := range d.ParentStates {
		if hint := ParentTopologyHint(parent); hint != "" {
			hints = appendUnique(hints, hint)
		}
	}
	return hints
}

func ParentTopologyHint(parent ParentDeviceState) string {
	joined := strings.ToUpper(strings.Join([]string{
		parent.InstanceID,
		parent.DisplayName,
		parent.Service,
		parent.Class,
		parent.Enumerator,
	}, " "))
	switch {
	case strings.Contains(joined, "USB4HOSTROUTER"):
		return "USB4 host router"
	case strings.Contains(joined, "USB4DEVICEROUTER"):
		return "USB4 device router"
	case strings.Contains(joined, "UCMUCSI") || strings.Contains(joined, " UCSI"):
		return "USB-C UCSI connector manager"
	case strings.Contains(joined, "THUNDERBOLT"):
		return "Thunderbolt path"
	case strings.Contains(joined, "USBXHCI"):
		return "USB xHCI host controller"
	case strings.Contains(joined, "USBHUB3") || strings.Contains(joined, "ROOT_HUB30"):
		return "USB 3 hub"
	default:
		return ""
	}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
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
	vidRe              = regexp.MustCompile(`(?i)\bVID[_-]?([0-9A-F]{4})\b`)
	pidRe              = regexp.MustCompile(`(?i)\bPID[_-]?([0-9A-F]{4})\b`)
	revRe              = regexp.MustCompile(`(?i)\bREV[_-]?([0-9A-F]{4})\b`)
	vidPidPlusSerialRe = regexp.MustCompile(`(?i)\bVID[_-]?[0-9A-F]{4}\+PID[_-]?[0-9A-F]{4}\+([^\\]+)`)
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
		d.Serial = serialFromInstanceID(d.InstanceID)
	}
	d.VID = strings.ToUpper(d.VID)
	d.PID = strings.ToUpper(d.PID)
	d.Revision = strings.ToUpper(d.Revision)
}

func serialFromInstanceID(instanceID string) string {
	if serial := firstMatch(vidPidPlusSerialRe, instanceID); serial != "" {
		return serial
	}
	parts := strings.Split(instanceID, `\`)
	if len(parts) > 2 {
		return parts[len(parts)-1]
	}
	return ""
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
