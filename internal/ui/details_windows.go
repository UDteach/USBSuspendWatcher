package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"usb-suspend-watch/internal/model"
)

func formatDevice(d model.DeviceSnapshot) string {
	lines := []string{
		"Device / デバイス",
		"Name / 名前: " + d.DisplayName(),
		"Instance ID: " + d.InstanceID,
		"Hardware ID: " + d.HardwareID,
		"VID/PID: " + d.VIDPID(),
		"Revision: " + d.Revision,
		"Serial: " + d.Serial,
		"Power state / 電源状態: " + string(d.PowerState),
		"Manufacturer / 製造元: " + d.Manufacturer,
		"Service: " + d.Service,
		"Class: " + d.Class,
		"Enumerator: " + d.Enumerator,
		"Location / 場所: " + d.Location,
		"Last seen / 最終確認: " + d.LastSeen.Format(time.RFC3339),
	}
	return strings.Join(lines, "\r\n")
}

func formatEvent(e model.Event) string {
	lines := []string{
		"Event / イベント",
		"Mark / 重要表示: " + eventMark(e),
		"Time / 時刻: " + e.Time.Format(time.RFC3339Nano),
		"Type / 種別: " + string(e.Type),
		"Source: " + string(e.Source),
		"Confidence / 信頼度: " + string(e.Confidence),
		"Message / メッセージ: " + e.Message,
		"Provider: " + e.Provider,
		fmt.Sprintf("Event ID: %d", e.EventID),
		"",
		formatDevice(e.Device),
	}
	if len(e.Raw) > 0 {
		lines = append(lines, "", "Raw ETW properties:")
		keys := make([]string, 0, len(e.Raw))
		for key := range e.Raw {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, key+": "+e.Raw[key])
		}
	}
	return strings.Join(lines, "\r\n")
}

func versionOrDev(v string) string {
	if v == "" {
		return "dev"
	}
	return v
}
