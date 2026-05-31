# Trace Buffering

PixRail supports OpenTelemetry traces locally through stdout or a noop provider. That is enough for this repository because there is no external collector dependency in scope.

## Current Local Behavior

- `PIXRAIL_TRACING_EXPORTER=stdout` emits spans to stdout for local inspection.
- `PIXRAIL_TRACING_EXPORTER=none` installs a noop tracer provider.
- API and worker use separate service names: `pixrail-api` and `pixrail-worker`.
- HTTP middleware extracts W3C trace context from request headers.

## Buffering Option For Production

If PixRail is deployed with real provider integrations, trace buffering should be outside the hot path:

```text
PixRail API/worker -> OpenTelemetry Collector -> Kafka/Redpanda -> backend exporter
```

Why buffer:

- Trace backends can be slower than payment traffic.
- Provider incidents often create trace bursts exactly when latency budget matters.
- Redpanda/Kafka lets operators retain telemetry during exporter outages.

What not to do:

- Do not publish traces synchronously from the transfer decision path.
- Do not block SPI submission or settlement callbacks on trace export.
- Do not put high-cardinality PII in span attributes.

## Local Gap

No Redpanda, Kafka, or OpenTelemetry Collector is started by Compose. That is intentional: it would make this backend challenge depend on telemetry infrastructure rather than proving the application instrumentation and operational design.
