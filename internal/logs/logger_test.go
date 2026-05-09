package logs

import (
	"strings"
	"testing"
	"time"

	"usb-suspend-watch/internal/model"
)

func TestReadEventsSinceHandlesLargeETWPayload(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewEventLoggerWithPrefix(dir, "large")
	if err != nil {
		t.Fatalf("NewEventLoggerWithPrefix returned error: %v", err)
	}
	large := strings.Repeat("x", 128*1024)
	if err := logger.Append(model.Event{
		Time:       time.Unix(10, 0),
		Type:       model.EventInfo,
		Source:     model.SourceApp,
		Confidence: model.ConfidenceLow,
		Message:    large,
	}); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	events, _, err := ReadEventsSince(logger.Path(), 0)
	if err != nil {
		t.Fatalf("ReadEventsSince returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("ReadEventsSince returned %d events, want 1", len(events))
	}
	if events[0].Message != large {
		t.Fatalf("large message was not preserved")
	}
}
