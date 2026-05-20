package netstats

import (
	"testing"
	"time"
)

func TestParseNetAdapterStatisticsJSON(t *testing.T) {
	input := []byte(`[{"Name":"USB NIC","InterfaceIndex":12,"Status":"Up","LinkSpeed":"1 Gbps","OutboundErrors":1,"InboundErrors":2,"OutboundDiscardedPackets":3,"InboundDiscardedPackets":4,"DeviceInstanceID":"USB\\VID_1234&PID_5678\\A"}]`)
	got, err := Parse("sabc12", time.Unix(10, 0), input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	row := got[0]
	if row.CorrelationID != "sabc12" || row.InterfaceIndex != "12" || row.OutboundErrors != "1" || row.InboundErrors != "2" {
		t.Fatalf("unexpected parsed row: %#v", row)
	}
	if row.DiscardedPackets != "out=3 in=4" {
		t.Fatalf("DiscardedPackets = %q", row.DiscardedPackets)
	}
}
