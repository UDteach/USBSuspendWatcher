package usbpcap

import (
	"reflect"
	"strings"
	"testing"

	"usb-suspend-watch/internal/model"
)

func TestParseInterfaces(t *testing.T) {
	got := ParseInterfaces("interface {value=\\\\.\\USBPcap1}{display=USBPcap1}\n")
	if len(got) != 1 || got[0].Value != `\\.\USBPcap1` || got[0].Display != "USBPcap1" {
		t.Fatalf("ParseInterfaces() = %#v", got)
	}
}

func TestParseConfigDevicesExtractsAddress(t *testing.T) {
	got := ParseConfigDevices("value {arg=99}{value=5_1}{display=USB Serial Port (COM52)}{enabled=false}{parent=5}\n")
	if len(got) != 1 {
		t.Fatalf("device count = %d", len(got))
	}
	if got[0].Address != 5 || got[0].Display != "USB Serial Port (COM52)" || got[0].Parent != "5" {
		t.Fatalf("unexpected device: %#v", got[0])
	}
}

func TestBuildPlanUsesMatchedDeviceAddress(t *testing.T) {
	interfaces := []Interface{
		{
			Value: `\\.\USBPcap1`,
			Devices: []Device{
				{Value: "2", Address: 2, Display: "[2] USB Composite Device"},
				{Value: "5_1", Address: 5, Display: "USB Serial Port (COM52)", Parent: "5"},
			},
		},
	}
	plan, err := BuildPlan("USBPcapCMD.exe", interfaces, model.DeviceSnapshot{
		FriendlyName: "USB Serial Port",
		COMPort:      "COM52",
	}, `C:\tmp\a.pcap`, `C:\tmp\a.json`)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan.CaptureAll {
		t.Fatalf("expected device-address capture, got capture-all plan: %#v", plan)
	}
	if !reflect.DeepEqual(plan.DeviceAddresses, []int{5}) {
		t.Fatalf("addresses = %#v", plan.DeviceAddresses)
	}
	if !strings.Contains(strings.Join(plan.Args, " "), "--devices 5") {
		t.Fatalf("args do not include device address: %#v", plan.Args)
	}
}

func TestBuildPlanFallsBackToRootHubCapture(t *testing.T) {
	plan, err := BuildPlan("USBPcapCMD.exe", []Interface{{Value: `\\.\USBPcap2`}}, model.DeviceSnapshot{
		FriendlyName: "USB Serial Port",
		COMPort:      "COM52",
	}, `C:\tmp\a.pcap`, `C:\tmp\a.json`)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if !plan.CaptureAll || plan.Warning == "" {
		t.Fatalf("expected capture-all fallback warning: %#v", plan)
	}
	if !strings.Contains(strings.Join(plan.Args, " "), "-A") {
		t.Fatalf("args do not include -A: %#v", plan.Args)
	}
}

func TestBuildPlanRefusesAmbiguousRootHubFallback(t *testing.T) {
	_, err := BuildPlan("USBPcapCMD.exe", []Interface{{Value: `\\.\USBPcap1`}, {Value: `\\.\USBPcap2`}}, model.DeviceSnapshot{
		FriendlyName: "USB Serial Port",
		COMPort:      "COM52",
	}, `C:\tmp\a.pcap`, `C:\tmp\a.json`)
	if err == nil || !strings.Contains(err.Error(), "refusing to guess") {
		t.Fatalf("expected ambiguous fallback error, got %v", err)
	}
}
