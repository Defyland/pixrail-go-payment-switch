# Production Readiness

PixRail is designed to be close to production without depending on real payment-network infrastructure. This file separates implemented local controls from honest external gaps.

## Implemented Locally

| Area | Evidence |
| --- | --- |
| Context propagation | HTTP handlers, switch service, stores, worker, migration runner, and adapters accept `context.Context`. |
| Graceful shutdown | API and worker handle `SIGINT`/`SIGTERM`; API drains HTTP server; tracing and pprof shutdown with deadlines. |
| Health/readiness | `/healthz` is liveness; `/readyz` checks store health. |
| Startup runtime log | API and worker log `GOMAXPROCS`, `NumCPU`, component, environment, and store driver. |
| Timeouts | HTTP server has `ReadHeaderTimeout`; DICT simulation has timeout; SPI submission has bounded timeout below claim TTL. |
| PostgreSQL pooling | Pool max/min/lifetime are explicit config. |
| Structured errors | Domain errors use sentinel values and wrapping with `errors.Is` handling in API. |
| Metrics | HTTP, decision, outbox, and runtime metrics are exposed in Prometheus format. |
| Tracing | OpenTelemetry trace provider is configurable per environment. |
| Benchmarks | API, k6, serialization, and rate limiter benchmarks are documented. |
| Binary payload contracts | Payment events and participant-profile cache codecs are executable, benchmarked, and tested for malformed payloads. |
| Optional pprof | `PIXRAIL_PPROF_ADDR` starts Go pprof endpoints for diagnostics. |
| Worker safety | SPI and outbox work use persisted claims/leases. |
| Callback authenticity | Provider callbacks require timestamped HMAC signatures. |

## Local Defaults

| Config | Default | Notes |
| --- | --- | --- |
| `PIXRAIL_STORE_DRIVER` | `memory` | Local only; production rejects memory mode. |
| `PIXRAIL_POSTGRES_MAX_CONNS` | `10` | API default; worker uses Compose default `5`. |
| `PIXRAIL_POSTGRES_MAX_CONN_LIFETIME` | `30m` | Prevents stale long-lived pool connections. |
| `PIXRAIL_WORKER_BATCH_SIZE` | `100` | SPI worker scan batch. |
| `PIXRAIL_WORKER_INTERVAL` | `1s` | SPI worker polling interval. |
| `PIXRAIL_PPROF_ADDR` | empty | Disabled unless explicitly enabled. |

## Honest Gaps

- Real DICT/SPI/antifraud providers are simulated. External adapters must add provider-specific idempotency keys, retries, circuit breaking, and contract tests.
- Broker-backed outbox publishing is represented by a relay interface and in-memory publisher, not a real Kafka/RabbitMQ adapter.
- Distributed rate limiting needs Redis or an equivalent shared store before horizontally scaling API replicas; local algorithms and Redis-suitable cache codecs are implemented, but no external cache is required to run PixRail.
- Kubernetes manifests are intentionally not included; runtime docs explain what must be checked for GOMAXPROCS, CPU throttling, p99, and pprof exposure.
- Trace buffering with Redpanda/Kafka is documented as a production option, not started in local Compose.

## Senior Review Position

The repository should be evaluated as a production-minded local payment switch reference, not as a certified Pix participant. The quality bar is application architecture, consistency boundaries, correctness tests, operational evidence, and performance discipline.
