package usb

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"usb-suspend-watch/internal/model"
)

func TestPowerStateFromCMDataReadsMostRecentPowerState(t *testing.T) {
	buf := make([]byte, 12)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(buf)))
	binary.LittleEndian.PutUint32(buf[4:8], 4)
	binary.LittleEndian.PutUint32(buf[8:12], 0xFFFFFFFF)

	if got := powerStateFromCMData(buf); got != model.PowerD3 {
		t.Fatalf("expected D3 from PD_MostRecentPowerState, got %s", got)
	}
}

func TestSplitMultiStringPropertyData(t *testing.T) {
	encoded := utf16.Encode([]rune("USB\\VID_1111&PID_2222\x00USB\\Class_03\x00\x00"))
	buf := make([]byte, len(encoded)*2)
	for i, v := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}

	got := splitUTF16MultiString(buf)
	if len(got) != 2 {
		t.Fatalf("expected two strings, got %#v", got)
	}
	if got[0] != `USB\VID_1111&PID_2222` || got[1] != `USB\Class_03` {
		t.Fatalf("unexpected strings: %#v", got)
	}
}

func TestListPresentDevicesSmoke(t *testing.T) {
	devices, err := ListPresentDevices()
	if err != nil {
		t.Fatalf("ListPresentDevices returned error: %v", err)
	}
	for _, device := range devices {
		if device.InstanceID == "" {
			t.Fatalf("device has empty instance id: %#v", device)
		}
	}
}

func TestIsUSBDeviceIncludesUSB3USB4TopologyNodes(t *testing.T) {
	cases := []model.DeviceSnapshot{
		{Service: "Usb4HostRouter", Description: "USB4 Host Router"},
		{Service: "Usb4DeviceRouter", Description: "USB4 Device Router"},
		{Service: "UcmUcsiCx", Description: "UCSI USB Connector Manager"},
		{Service: "USBXHCI", Description: "USB xHCI Compliant Host Controller"},
		{Service: "USBHUB3", Description: "USB Root Hub (USB 3.0)"},
	}
	for _, device := range cases {
		if !isUSBDevice(device) {
			t.Fatalf("expected topology node to be included: %#v", device)
		}
	}
}
