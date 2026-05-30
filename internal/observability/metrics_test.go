package observability

import (
	"strings"
	"testing"
	"time"
)

func TestMetricsWritePrometheus(t *testing.T) {
	metrics := NewMetrics()
	metrics.ObserveRequest("POST", "/v1/pix/transfers", 201, 2*time.Millisecond)
	metrics.ObserveDecision("approved")
	metrics.ObserveEvents(5)

	var out strings.Builder
	metrics.WritePrometheus(&out)
	text := out.String()
	for _, expected := range []string{
		"pixrail_http_requests_total",
		"pixrail_transfer_decisions_total",
		"pixrail_outbox_events_total",
		"pixrail_http_request_latency_seconds",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected metric %s in output:\n%s", expected, text)
		}
	}
}
