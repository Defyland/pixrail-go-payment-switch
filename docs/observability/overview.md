# Observability

PixRail emits three operational signal types:

- structured JSON logs with route, status, duration, request ID, and correlation ID
- Prometheus text metrics at `/metrics`
- OpenTelemetry HTTP route spans with trace-context propagation

## Core metrics

- `pixrail_http_requests_total`
- `pixrail_http_request_latency_seconds`
- `pixrail_transfer_decisions_total`
- `pixrail_outbox_events_total`
- `pixrail_uptime_seconds`

## Dashboard

Grafana dashboard JSON is stored at `observability/grafana/pixrail-overview-dashboard.json`.
