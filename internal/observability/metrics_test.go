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
		"pixrail_runtime_gomaxprocs",
		"pixrail_runtime_num_cpu",
		"pixrail_runtime_goroutines",
		"pixrail_runtime_heap_alloc_bytes",
		"pixrail_transfer_decisions_total",
		"pixrail_outbox_events_total",
		"pixrail_http_request_latency_seconds",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected metric %s in output:\n%s", expected, text)
		}
	}
}

func TestMetricsBoundsLatencySamplesPerRoute(t *testing.T) {
	metrics := NewMetrics()
	for i := 0; i < maxLatencySamplesPerRoute+10; i++ {
		metrics.ObserveRequest("POST", "/v1/pix/transfers", 201, time.Duration(i)*time.Millisecond)
	}

	var out strings.Builder
	metrics.WritePrometheus(&out)
	if !strings.Contains(out.String(), `pixrail_http_request_latency_seconds_count{method="POST",route="/v1/pix/transfers"} 1024`) {
		t.Fatalf("expected bounded latency sample count, got:\n%s", out.String())
	}
}
