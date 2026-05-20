package netstats

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"usb-suspend-watch/internal/model"
)

type adapterJSON struct {
	Name                     string      `json:"Name"`
	InterfaceIndex           interface{} `json:"InterfaceIndex"`
	Status                   string      `json:"Status"`
	LinkSpeed                string      `json:"LinkSpeed"`
	OutboundErrors           interface{} `json:"OutboundErrors"`
	InboundErrors            interface{} `json:"InboundErrors"`
	OutboundDiscardedPackets interface{} `json:"OutboundDiscardedPackets"`
	InboundDiscardedPackets  interface{} `json:"InboundDiscardedPackets"`
	DeviceInstanceID         string      `json:"DeviceInstanceID"`
}

func IsNetworkDevice(d model.DeviceSnapshot) bool {
	joined := strings.ToLower(strings.Join([]string{
		d.Class,
		d.Service,
		d.Description,
		d.FriendlyName,
		d.BusReportedDeviceDesc,
		d.InstanceID,
	}, " "))
	return strings.Contains(joined, "net") ||
		strings.Contains(joined, "ndis") ||
		strings.Contains(joined, "rndis") ||
		strings.Contains(joined, "wwan") ||
		strings.Contains(joined, "ethernet")
}

func Capture(ctx context.Context, correlationID string, t time.Time) ([]model.NetAdapterSnapshot, error) {
	if t.IsZero() {
		t = time.Now()
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	script := `$ErrorActionPreference='Stop'; Get-NetAdapter | ForEach-Object { $s=$null; $oe=$null; $ie=$null; $od=$null; $id=$null; try { $s=Get-NetAdapterStatistics -Name $_.Name } catch {}; if($s){ $oe=$s.OutboundErrors; $ie=$s.InboundErrors; $od=$s.OutboundDiscardedPackets; $id=$s.InboundDiscardedPackets }; [pscustomobject]@{ Name=$_.Name; InterfaceIndex=$_.InterfaceIndex; Status=$_.Status; LinkSpeed=$_.LinkSpeed; OutboundErrors=$oe; InboundErrors=$ie; OutboundDiscardedPackets=$od; InboundDiscardedPackets=$id; DeviceInstanceID=$_.PnPDeviceID } } | ConvertTo-Json -Compress`
	out, err := exec.CommandContext(cmdCtx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script).CombinedOutput()
	if err != nil {
		return nil, err
	}
	return Parse(correlationID, t, out)
}

func Parse(correlationID string, t time.Time, data []byte) ([]model.NetAdapterSnapshot, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	var rows []adapterJSON
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &rows); err != nil {
			return nil, err
		}
	} else {
		var row adapterJSON
		if err := json.Unmarshal([]byte(trimmed), &row); err != nil {
			return nil, err
		}
		rows = []adapterJSON{row}
	}
	out := make([]model.NetAdapterSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.NetAdapterSnapshot{
			Time:             t,
			CorrelationID:    correlationID,
			Name:             row.Name,
			InterfaceIndex:   scalarString(row.InterfaceIndex),
			Status:           row.Status,
			LinkSpeed:        row.LinkSpeed,
			OutboundErrors:   scalarString(row.OutboundErrors),
			InboundErrors:    scalarString(row.InboundErrors),
			DiscardedPackets: discardedPackets(row),
			DeviceInstanceID: row.DeviceInstanceID,
		})
	}
	return out, nil
}

func discardedPackets(row adapterJSON) string {
	outbound := scalarString(row.OutboundDiscardedPackets)
	inbound := scalarString(row.InboundDiscardedPackets)
	switch {
	case outbound != "" && inbound != "":
		return "out=" + outbound + " in=" + inbound
	case outbound != "":
		return "out=" + outbound
	case inbound != "":
		return "in=" + inbound
	default:
		return ""
	}
}

func scalarString(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strings.TrimRight(strings.TrimRight(jsonNumber(x), "0"), ".")
	default:
		return strings.TrimSpace(strings.Trim(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(toJSON(x)), "\r", ""), "\n", ""), `"`))
	}
}

func jsonNumber(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
