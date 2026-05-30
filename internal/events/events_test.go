package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewBuildsEnvelope(t *testing.T) {
	event, err := New("evt_1", "pix_transfer_requested", "tenant_a", "acct_1", "pxt_1", "corr_1", time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC), map[string]string{"status": "approved"})
	if err != nil {
		t.Fatalf("new event failed: %v", err)
	}
	if event.Producer != Producer || event.SchemaVersion != "1" || event.EventType != "pix_transfer_requested" {
		t.Fatalf("unexpected envelope: %+v", event)
	}
	var payload map[string]string
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("payload should be json: %v", err)
	}
	if payload["status"] != "approved" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}
