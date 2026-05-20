package model

import "testing"

func TestLooksLikeFTDISerialRequiresCOMPortAndFTDISignal(t *testing.T) {
	device := DeviceSnapshot{
		FriendlyName: "USB Serial Port",
		VID:          "0403",
		PID:          "6001",
		COMPort:      "COM52",
	}
	if !device.LooksLikeFTDISerial() {
		t.Fatalf("FTDI USB serial port should match target filter")
	}

	device.COMPort = ""
	if device.LooksLikeFTDISerial() {
		t.Fatalf("FTDI signal without COM port should not match FTDI COM target")
	}
}

func TestEnrichDeviceRelationshipsGroupsPortAndConverterBySerial(t *testing.T) {
	devices := EnrichDeviceRelationships([]DeviceSnapshot{
		{
			InstanceID:  `FTDIBUS\VID_0403+PID_6001+FT123\0000`,
			Description: "USB Serial Converter",
			HardwareID:  `FTDIBUS\VID_0403+PID_6001`,
			VID:         "0403",
			PID:         "6001",
			Serial:      "FT123",
			PowerState:  PowerD0,
		},
		{
			InstanceID:   `FTDIPORT\VID_0403+PID_6001+FT123\0000`,
			FriendlyName: "USB Serial Port (COM52)",
			HardwareID:   `FTDIPORT\VID_0403+PID_6001`,
			VID:          "0403",
			PID:          "6001",
			Serial:       "FT123",
			COMPort:      "COM52",
			PowerState:   PowerD0,
		},
	})

	if devices[0].LogicalGroupID == "" || devices[0].LogicalGroupID != devices[1].LogicalGroupID {
		t.Fatalf("devices should share logical group: %#v", devices)
	}
	if devices[0].RelationRole != "converter" || devices[1].RelationRole != "port" {
		t.Fatalf("unexpected relation roles: %#v", devices)
	}
	if len(devices[0].RelatedInstanceIDs) != 1 || len(devices[1].RelatedInstanceIDs) != 1 {
		t.Fatalf("related instance ids should be populated: %#v", devices)
	}
	if devices[0].DiagnosticScore != 90 || devices[1].DiagnosticScore != 90 {
		t.Fatalf("serial match should be high score: %#v", devices)
	}
	if devices[0].GroupDisplayName != "FTDI Adapter FT123" {
		t.Fatalf("group display name = %q, want FTDI Adapter FT123", devices[0].GroupDisplayName)
	}
}

func TestPopulateUSBIDsExtractsFTDISerialBeforeChildSuffix(t *testing.T) {
	device := DeviceSnapshot{
		InstanceID: `FTDIBUS\VID_0403+PID_6001+FT123\0000`,
	}
	PopulateUSBIDs(&device)
	if device.Serial != "FT123" {
		t.Fatalf("Serial = %q, want FT123", device.Serial)
	}
}

func TestEnrichDeviceRelationshipsDoesNotGroupByVIDPIDOnly(t *testing.T) {
	devices := EnrichDeviceRelationships([]DeviceSnapshot{
		{InstanceID: `USB\VID_0403&PID_6001\A`, VID: "0403", PID: "6001"},
		{InstanceID: `USB\VID_0403&PID_6001\B`, VID: "0403", PID: "6001"},
	})

	if devices[0].LogicalGroupID != "" || devices[1].LogicalGroupID != "" {
		t.Fatalf("VID/PID only must not create logical groups: %#v", devices)
	}
	if devices[0].DiagnosticScore != 0 || devices[1].DiagnosticScore != 0 {
		t.Fatalf("VID/PID only must not create confidence score: %#v", devices)
	}
}

func TestEnrichDeviceRelationshipsFlagsParentLowPowerChildD0(t *testing.T) {
	devices := EnrichDeviceRelationships([]DeviceSnapshot{
		{
			InstanceID:         `USB\VID_0403&PID_6001\FT123`,
			ParentInstanceID:   `USB\ROOT_HUB30\1`,
			ParentChain:        []string{`USB\ROOT_HUB30\1`},
			VID:                "0403",
			PID:                "6001",
			COMPort:            "COM52",
			PowerState:         PowerD0,
			PowerStateEvidence: "child D0",
		},
		{
			InstanceID:         `USB\ROOT_HUB30\1`,
			FriendlyName:       "USB Root Hub",
			PowerState:         PowerD3,
			PowerStateEvidence: "parent D3",
		},
	})

	if !devices[0].ParentLowPowerChildD0 {
		t.Fatalf("child D0 with parent D3 should be flagged: %#v", devices[0])
	}
	if got := devices[0].ParentStates[0].PowerState; got != PowerD3 {
		t.Fatalf("parent power state = %s, want D3", got)
	}
}
