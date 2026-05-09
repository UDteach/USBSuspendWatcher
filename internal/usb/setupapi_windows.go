package usb

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"usb-suspend-watch/internal/model"
)

const (
	digcfPresent    = 0x00000002
	digcfAllClasses = 0x00000004

	spdrpDeviceDesc      = 0x00000000
	spdrpHardwareID      = 0x00000001
	spdrpService         = 0x00000004
	spdrpClass           = 0x00000007
	spdrpMFG             = 0x0000000B
	spdrpFriendlyName    = 0x0000000C
	spdrpLocationInfo    = 0x0000000D
	spdrpEnumeratorName  = 0x00000016
	spdrpDevicePowerData = 0x0000001E
)

var (
	setupapi                         = windows.NewLazySystemDLL("setupapi.dll")
	procSetupDiGetClassDevsW         = setupapi.NewProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInfo        = setupapi.NewProc("SetupDiEnumDeviceInfo")
	procSetupDiDestroyDeviceInfoList = setupapi.NewProc("SetupDiDestroyDeviceInfoList")
	procSetupDiGetDeviceRegistryProp = setupapi.NewProc("SetupDiGetDeviceRegistryPropertyW")
	procSetupDiGetDeviceInstanceIDW  = setupapi.NewProc("SetupDiGetDeviceInstanceIdW")
)

type spDevInfoData struct {
	cbSize    uint32
	classGUID windows.GUID
	devInst   uint32
	reserved  uintptr
}

func ListPresentDevices() ([]model.DeviceSnapshot, error) {
	h, _, err := procSetupDiGetClassDevsW.Call(0, 0, 0, digcfPresent|digcfAllClasses)
	if h == uintptr(windows.InvalidHandle) {
		return nil, fmt.Errorf("SetupDiGetClassDevsW: %w", err)
	}
	defer procSetupDiDestroyDeviceInfoList.Call(h)

	now := time.Now()
	var devices []model.DeviceSnapshot
	for index := uint32(0); ; index++ {
		data := spDevInfoData{cbSize: uint32(unsafe.Sizeof(spDevInfoData{}))}
		ok, _, callErr := procSetupDiEnumDeviceInfo.Call(h, uintptr(index), uintptr(unsafe.Pointer(&data)))
		if ok == 0 {
			if callErr == windows.ERROR_NO_MORE_ITEMS {
				break
			}
			return devices, fmt.Errorf("SetupDiEnumDeviceInfo(%d): %w", index, callErr)
		}

		instanceID, _ := deviceInstanceID(h, &data)
		d := model.DeviceSnapshot{
			InstanceID:   instanceID,
			Description:  firstStringProperty(h, &data, spdrpDeviceDesc),
			FriendlyName: firstStringProperty(h, &data, spdrpFriendlyName),
			Manufacturer: firstStringProperty(h, &data, spdrpMFG),
			Service:      firstStringProperty(h, &data, spdrpService),
			Class:        firstStringProperty(h, &data, spdrpClass),
			Enumerator:   firstStringProperty(h, &data, spdrpEnumeratorName),
			Location:     firstStringProperty(h, &data, spdrpLocationInfo),
			HardwareID:   strings.Join(multiStringProperty(h, &data, spdrpHardwareID), ";"),
			PowerState:   powerStateProperty(h, &data),
			Present:      true,
			LastSeen:     now,
		}
		model.PopulateUSBIDs(&d)
		if isUSBDevice(d) {
			devices = append(devices, d)
		}
	}
	return devices, nil
}

func deviceInstanceID(h uintptr, data *spDevInfoData) (string, error) {
	var required uint32
	procSetupDiGetDeviceInstanceIDW.Call(h, uintptr(unsafe.Pointer(data)), 0, 0, uintptr(unsafe.Pointer(&required)))
	if required == 0 {
		return "", windows.GetLastError()
	}
	buf := make([]uint16, required)
	ok, _, err := procSetupDiGetDeviceInstanceIDW.Call(
		h,
		uintptr(unsafe.Pointer(data)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(required),
		uintptr(unsafe.Pointer(&required)),
	)
	if ok == 0 {
		return "", err
	}
	return windows.UTF16ToString(buf), nil
}

func firstStringProperty(h uintptr, data *spDevInfoData, prop uint32) string {
	values := multiStringProperty(h, data, prop)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func multiStringProperty(h uintptr, data *spDevInfoData, prop uint32) []string {
	buf, ok := registryProperty(h, data, prop)
	if !ok || len(buf) < 2 {
		return nil
	}
	return splitUTF16MultiString(buf)
}

func splitUTF16MultiString(buf []byte) []string {
	if len(buf) < 2 {
		return nil
	}
	u16 := make([]uint16, len(buf)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(buf[i*2:])
	}
	var out []string
	start := 0
	for i, r := range u16 {
		if r == 0 {
			if i > start {
				out = append(out, windows.UTF16ToString(u16[start:i]))
			}
			start = i + 1
		}
	}
	if len(out) == 0 {
		if s := windows.UTF16ToString(u16); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func powerStateProperty(h uintptr, data *spDevInfoData) model.DevicePowerState {
	buf, ok := registryProperty(h, data, spdrpDevicePowerData)
	if !ok {
		return model.PowerUnknown
	}
	return powerStateFromCMData(buf)
}

func powerStateFromCMData(buf []byte) model.DevicePowerState {
	if len(buf) < 8 {
		return model.PowerUnknown
	}
	state := binary.LittleEndian.Uint32(buf[4:8])
	switch state {
	case 1:
		return model.PowerD0
	case 2:
		return model.PowerD1
	case 3:
		return model.PowerD2
	case 4:
		return model.PowerD3
	default:
		return model.PowerUnknown
	}
}

func registryProperty(h uintptr, data *spDevInfoData, prop uint32) ([]byte, bool) {
	var dataType uint32
	var required uint32
	ok, _, err := procSetupDiGetDeviceRegistryProp.Call(
		h,
		uintptr(unsafe.Pointer(data)),
		uintptr(prop),
		uintptr(unsafe.Pointer(&dataType)),
		0,
		0,
		uintptr(unsafe.Pointer(&required)),
	)
	if ok == 0 && required == 0 {
		_ = err
		return nil, false
	}
	if required == 0 {
		return nil, false
	}
	buf := make([]byte, required)
	ok, _, _ = procSetupDiGetDeviceRegistryProp.Call(
		h,
		uintptr(unsafe.Pointer(data)),
		uintptr(prop),
		uintptr(unsafe.Pointer(&dataType)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&required)),
	)
	return buf, ok != 0
}

func isUSBDevice(d model.DeviceSnapshot) bool {
	inst := strings.ToUpper(d.InstanceID)
	enum := strings.ToUpper(d.Enumerator)
	hw := strings.ToUpper(d.HardwareID)
	if strings.HasPrefix(inst, `BTHENUM\`) {
		return false
	}
	if enum == "USB" || enum == "USBSTOR" || strings.HasPrefix(inst, `USB\`) || strings.HasPrefix(inst, `USBSTOR\`) {
		return true
	}
	return strings.Contains(hw, "VID_") && strings.Contains(hw, "PID_")
}
