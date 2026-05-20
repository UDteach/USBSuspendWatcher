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

func TestPowerDataFromCMDataDecodesMajorFields(t *testing.T) {
	buf := make([]byte, 56)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(buf)))
	binary.LittleEndian.PutUint32(buf[4:8], 4)
	binary.LittleEndian.PutUint32(buf[8:12], 0x1234)
	binary.LittleEndian.PutUint32(buf[12:16], 11)
	binary.LittleEndian.PutUint32(buf[16:20], 22)
	binary.LittleEndian.PutUint32(buf[20:24], 33)
	for i := 0; i < 7; i++ {
		binary.LittleEndian.PutUint32(buf[24+i*4:28+i*4], uint32((i%4)+1))
	}
	binary.LittleEndian.PutUint32(buf[52:56], 3)

	got := powerDataFromCMData(buf)
	if got.MostRecentPowerState != model.PowerD3 || got.MostRecentPowerStateRaw != 4 {
		t.Fatalf("unexpected most recent state: %#v", got)
	}
	if got.Capabilities != 0x1234 || got.D1Latency != 11 || got.D2Latency != 22 || got.D3Latency != 33 {
		t.Fatalf("latency/capability fields were not decoded: %#v", got)
	}
	if len(got.PowerStateMapping) != 7 || got.PowerStateMapping[0] != model.PowerD0 || got.PowerStateMapping[3] != model.PowerD3 {
		t.Fatalf("power mapping not decoded: %#v", got.PowerStateMapping)
	}
	if got.DeepestSystemWake != "PowerSystemSleeping2" || got.DeepestSystemWakeRaw != 3 {
		t.Fatalf("deepest wake not decoded: %#v", got)
	}
	if got.D3HotColdNote == "" {
		t.Fatalf("D3hot/D3cold note should be present")
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
