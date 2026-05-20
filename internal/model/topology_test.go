package model

import "testing"

func TestTopologyHintsIncludeUSB3USB4AndTypeC(t *testing.T) {
	device := DeviceSnapshot{ParentStates: []ParentDeviceState{
		{Service: "USBHUB3", DisplayName: "USB Root Hub (USB 3.0)"},
		{Service: "Usb4DeviceRouter", DisplayName: "USB4 Device Router"},
		{Service: "UcmUcsiCx", DisplayName: "UCSI USB Connector Manager"},
	}}

	hints := TopologyHints(device)
	for _, want := range []string{"USB 3 hub", "USB4 device router", "USB-C UCSI connector manager"} {
		if !containsString(hints, want) {
			t.Fatalf("TopologyHints missing %q: %#v", want, hints)
		}
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
