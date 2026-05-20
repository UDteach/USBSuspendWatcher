package usb

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"usb-suspend-watch/internal/model"
)

const (
	digcfPresent    = 0x00000002
	digcfAllClasses = 0x00000004

	spdrpDeviceDesc               = 0x00000000
	spdrpHardwareID               = 0x00000001
	spdrpService                  = 0x00000004
	spdrpClass                    = 0x00000007
	spdrpMFG                      = 0x0000000B
	spdrpFriendlyName             = 0x0000000C
	spdrpLocationInfo             = 0x0000000D
	spdrpPhysicalDeviceObjectName = 0x0000000E
	spdrpEnumeratorName           = 0x00000016
	spdrpDevicePowerData          = 0x0000001E
	spdrpLocationPaths            = 0x00000023

	dicsFlagGlobal = 0x00000001
	diregDev       = 0x00000001
	keyRead        = 0x00020019
	crSuccess      = 0
)

var (
	setupapi                         = windows.NewLazySystemDLL("setupapi.dll")
	cfgmgr32                         = windows.NewLazySystemDLL("cfgmgr32.dll")
	procSetupDiGetClassDevsW         = setupapi.NewProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInfo        = setupapi.NewProc("SetupDiEnumDeviceInfo")
	procSetupDiDestroyDeviceInfoList = setupapi.NewProc("SetupDiDestroyDeviceInfoList")
	procSetupDiGetDeviceRegistryProp = setupapi.NewProc("SetupDiGetDeviceRegistryPropertyW")
	procSetupDiGetDeviceInstanceIDW  = setupapi.NewProc("SetupDiGetDeviceInstanceIdW")
	procSetupDiOpenDevRegKey         = setupapi.NewProc("SetupDiOpenDevRegKey")
	procCMGetParent                  = cfgmgr32.NewProc("CM_Get_Parent")
	procCMGetDeviceIDW               = cfgmgr32.NewProc("CM_Get_Device_IDW")

	comPortRe = regexp.MustCompile(`\((COM\d+)\)`)
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
		powerState, powerEvidence, powerHex := powerStateProperty(h, &data)
		parentChain := parentInstanceChain(data.devInst)
		d := model.DeviceSnapshot{
			InstanceID:               instanceID,
			Description:              firstStringProperty(h, &data, spdrpDeviceDesc),
			FriendlyName:             firstStringProperty(h, &data, spdrpFriendlyName),
			Manufacturer:             firstStringProperty(h, &data, spdrpMFG),
			Service:                  firstStringProperty(h, &data, spdrpService),
			Class:                    firstStringProperty(h, &data, spdrpClass),
			Enumerator:               firstStringProperty(h, &data, spdrpEnumeratorName),
			Location:                 firstStringProperty(h, &data, spdrpLocationInfo),
			LocationPaths:            multiStringProperty(h, &data, spdrpLocationPaths),
			HardwareID:               strings.Join(multiStringProperty(h, &data, spdrpHardwareID), ";"),
			COMPort:                  deviceCOMPort(h, &data),
			PhysicalDeviceObjectName: firstStringProperty(h, &data, spdrpPhysicalDeviceObjectName),
			ParentChain:              parentChain,
			PowerState:               powerState,
			PowerStateEvidence:       powerEvidence,
			PowerDataHex:             powerHex,
			Present:                  true,
			LastSeen:                 now,
		}
		if len(parentChain) > 0 {
			d.ParentInstanceID = parentChain[0]
		}
		model.PopulateUSBIDs(&d)
		if isUSBDevice(d) {
			devices = append(devices, d)
		}
	}
	return model.EnrichDeviceRelationships(devices), nil
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

func powerStateProperty(h uintptr, data *spDevInfoData) (model.DevicePowerState, string, string) {
	buf, ok := registryProperty(h, data, spdrpDevicePowerData)
	if !ok {
		return model.PowerUnknown, "SPDRP_DEVICE_POWER_DATA unavailable", ""
	}
	state, evidence := powerStateInfoFromCMData(buf)
	return state, evidence, strings.ToUpper(hex.EncodeToString(buf))
}

func powerStateFromCMData(buf []byte) model.DevicePowerState {
	state, _ := powerStateInfoFromCMData(buf)
	return state
}

func powerStateInfoFromCMData(buf []byte) (model.DevicePowerState, string) {
	if len(buf) < 8 {
		return model.PowerUnknown, fmt.Sprintf("SPDRP_DEVICE_POWER_DATA too short: %d bytes", len(buf))
	}
	state := binary.LittleEndian.Uint32(buf[4:8])
	switch state {
	case 1:
		return model.PowerD0, "SPDRP_DEVICE_POWER_DATA CM_POWER_DATA.PD_MostRecentPowerState=1 (D0)"
	case 2:
		return model.PowerD1, "SPDRP_DEVICE_POWER_DATA CM_POWER_DATA.PD_MostRecentPowerState=2 (D1)"
	case 3:
		return model.PowerD2, "SPDRP_DEVICE_POWER_DATA CM_POWER_DATA.PD_MostRecentPowerState=3 (D2)"
	case 4:
		return model.PowerD3, "SPDRP_DEVICE_POWER_DATA CM_POWER_DATA.PD_MostRecentPowerState=4 (D3)"
	default:
		return model.PowerUnknown, fmt.Sprintf("SPDRP_DEVICE_POWER_DATA CM_POWER_DATA.PD_MostRecentPowerState=%d (unknown)", state)
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

func deviceCOMPort(h uintptr, data *spDevInfoData) string {
	if port := deviceRegistryString(h, data, "PortName"); port != "" {
		return port
	}
	if name := firstStringProperty(h, data, spdrpFriendlyName); name != "" {
		if match := comPortRe.FindStringSubmatch(name); len(match) == 2 {
			return match[1]
		}
	}
	return ""
}

func deviceRegistryString(h uintptr, data *spDevInfoData, name string) string {
	key, _, _ := procSetupDiOpenDevRegKey.Call(
		h,
		uintptr(unsafe.Pointer(data)),
		dicsFlagGlobal,
		0,
		diregDev,
		keyRead,
	)
	if key == uintptr(windows.InvalidHandle) || key == 0 {
		return ""
	}
	defer windows.RegCloseKey(windows.Handle(key))

	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return ""
	}
	var typ uint32
	var required uint32
	err = windows.RegQueryValueEx(windows.Handle(key), namePtr, nil, &typ, nil, &required)
	if err != nil || required == 0 {
		return ""
	}
	buf := make([]byte, required)
	if err := windows.RegQueryValueEx(windows.Handle(key), namePtr, nil, &typ, &buf[0], &required); err != nil {
		return ""
	}
	if len(buf) < 2 {
		return ""
	}
	u16 := make([]uint16, len(buf)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(buf[i*2:])
	}
	return windows.UTF16ToString(u16)
}

func parentInstanceChain(devInst uint32) []string {
	var out []string
	current := devInst
	for i := 0; i < 16; i++ {
		var parent uint32
		ret, _, _ := procCMGetParent.Call(uintptr(unsafe.Pointer(&parent)), uintptr(current), 0)
		if ret != crSuccess || parent == 0 {
			break
		}
		id := deviceIDForDevInst(parent)
		if id == "" {
			break
		}
		out = append(out, id)
		current = parent
	}
	return out
}

func deviceIDForDevInst(devInst uint32) string {
	buf := make([]uint16, 1024)
	ret, _, _ := procCMGetDeviceIDW.Call(
		uintptr(devInst),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		0,
	)
	if ret != crSuccess {
		return ""
	}
	return windows.UTF16ToString(buf)
}

func isUSBDevice(d model.DeviceSnapshot) bool {
	inst := strings.ToUpper(d.InstanceID)
	enum := strings.ToUpper(d.Enumerator)
	hw := strings.ToUpper(d.HardwareID)
	service := strings.ToUpper(d.Service)
	name := strings.ToUpper(strings.Join([]string{d.FriendlyName, d.Description, d.Class}, " "))
	if strings.HasPrefix(inst, `BTHENUM\`) {
		return false
	}
	if enum == "USB" || enum == "USBSTOR" || strings.HasPrefix(inst, `USB\`) || strings.HasPrefix(inst, `USBSTOR\`) {
		return true
	}
	if strings.Contains(hw, "VID_") && strings.Contains(hw, "PID_") {
		return true
	}
	topologyText := strings.Join([]string{inst, service, name}, " ")
	return strings.Contains(topologyText, "USB4HOSTROUTER") ||
		strings.Contains(topologyText, "USB4DEVICEROUTER") ||
		strings.Contains(topologyText, "UCMUCSI") ||
		strings.Contains(topologyText, "USBXHCI") ||
		strings.Contains(topologyText, "USBHUB3") ||
		strings.Contains(topologyText, "ROOT_HUB30") ||
		strings.Contains(topologyText, "THUNDERBOLT")
}
