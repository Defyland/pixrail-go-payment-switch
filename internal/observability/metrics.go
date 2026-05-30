package observability

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

type Metrics struct {
	mu        sync.Mutex
	requests  map[string]int64
	latency   map[string][]float64
	decisions map[string]int64
	events    map[string]int64
	started   time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{
		requests:  make(map[string]int64),
		latency:   make(map[string][]float64),
		decisions: make(map[string]int64),
		events:    make(map[string]int64),
		started:   time.Now().UTC(),
	}
}

func (m *Metrics) ObserveRequest(method, route string, status int, elapsed time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf(`method="%s",route="%s",status="%d"`, method, route, status)
	m.requests[key]++
	latencyKey := fmt.Sprintf(`method="%s",route="%s"`, method, route)
	m.latency[latencyKey] = append(m.latency[latencyKey], elapsed.Seconds())
}

func (m *Metrics) ObserveDecision(status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decisions[status]++
}

func (m *Metrics) ObserveEvents(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events["outbox_inserted"] += int64(count)
}

func (m *Metrics) WritePrometheus(w io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	writeMetricHeader(w, "pixrail_uptime_seconds", "gauge", "Seconds since the process started.")
	fmt.Fprintf(w, "pixrail_uptime_seconds %.0f\n", time.Since(m.started).Seconds())
	writeMetricHeader(w, "pixrail_http_requests_total", "counter", "HTTP requests by method, route, and status.")
	writeCounterMap(w, "pixrail_http_requests_total", m.requests)
	writeMetricHeader(w, "pixrail_transfer_decisions_total", "counter", "Pix transfer decisions by status.")
	writeCounterMap(w, "pixrail_transfer_decisions_total", labelValues(m.decisions, "status"))
	writeMetricHeader(w, "pixrail_outbox_events_total", "counter", "Outbox events created by the switch.")
	writeCounterMap(w, "pixrail_outbox_events_total", labelValues(m.events, "kind"))
	writeMetricHeader(w, "pixrail_http_request_latency_seconds", "summary", "Observed HTTP request latency samples.")
	writeSummaryMap(w, "pixrail_http_request_latency_seconds", m.latency)
}

func writeMetricHeader(w io.Writer, name, metricType, help string) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, metricType)
}

func writeCounterMap(w io.Writer, name string, values map[string]int64) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if strings.Contains(key, "=") {
			fmt.Fprintf(w, "%s{%s} %d\n", name, key, values[key])
		} else {
			fmt.Fprintf(w, "%s{%s} %d\n", name, key, values[key])
		}
	}
}

func writeSummaryMap(w io.Writer, name string, values map[string][]float64) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		samples := append([]float64(nil), values[key]...)
		sort.Float64s(samples)
		if len(samples) == 0 {
			continue
		}
		fmt.Fprintf(w, "%s_count{%s} %d\n", name, key, len(samples))
		fmt.Fprintf(w, "%s_sum{%s} %.6f\n", name, key, sum(samples))
		fmt.Fprintf(w, "%s{quantile=\"0.50\",%s} %.6f\n", name, key, percentile(samples, 0.50))
		fmt.Fprintf(w, "%s{quantile=\"0.95\",%s} %.6f\n", name, key, percentile(samples, 0.95))
		fmt.Fprintf(w, "%s{quantile=\"0.99\",%s} %.6f\n", name, key, percentile(samples, 0.99))
	}
}

func labelValues(values map[string]int64, label string) map[string]int64 {
	out := make(map[string]int64, len(values))
	for key, value := range values {
		out[fmt.Sprintf(`%s="%s"`, label, key)] = value
	}
	return out
}

func percentile(samples []float64, quantile float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	index := int(float64(len(samples)-1) * quantile)
	return samples[index]
}

func sum(samples []float64) float64 {
	var total float64
	for _, sample := range samples {
		total += sample
	}
	return total
}
