# Deployment Readiness

PixRail will eventually run multiple hot-path components: API, DICT resolver, rate limiter, fraud scoring, SPI gateway, event publisher, and analytics projection.

## Current posture

- PixRail ships a runnable Go HTTP API with in-memory adapters.
- Health, readiness, Prometheus metrics, structured logs, and OpenTelemetry spans are implemented.
- k6 scripts and native Go benchmarks are included for smoke, load, stress, and spike validation.
- PostgreSQL, Redis, a broker relay, and ClickHouse remain production-hardening adapters, not MVP blockers.

## Deferred platform work

- Kubernetes manifests are deferred until persistent provider boundaries are implemented.
- Flink is deferred to advanced analytics and does not block the initial payment switch.
- Service mesh is deferred; application-level idempotency, rate limiting, and tracing come first.
