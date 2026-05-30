# Deployment Readiness

PixRail will eventually run multiple hot-path components: API, DICT resolver, rate limiter, fraud scoring, SPI gateway, event publisher, and analytics projection.

## Current posture

- PixRail ships a runnable Go HTTP API with in-memory and PostgreSQL storage adapters.
- Health, readiness, Prometheus metrics, structured logs, and OpenTelemetry spans are implemented.
- k6 scripts and native Go benchmarks are included for smoke, load, stress, and spike validation.
- PostgreSQL persistence and migration runner are implemented.
- Redis-backed distributed rate limiting, a real broker publisher, and ClickHouse remain production-hardening adapters, not MVP blockers.

## Deferred platform work

- Kubernetes manifests are deferred until persistent provider boundaries are implemented.
- Flink is deferred to advanced analytics and does not block the initial payment switch.
- Service mesh is deferred; application-level idempotency, rate limiting, and tracing come first.
