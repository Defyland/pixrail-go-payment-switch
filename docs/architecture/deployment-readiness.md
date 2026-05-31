# Deployment Readiness

PixRail will eventually run multiple hot-path components: API, DICT resolver, rate limiter, fraud scoring, SPI gateway, event publisher, and analytics projection.

## Current posture

- PixRail ships a runnable Go HTTP API with in-memory and PostgreSQL storage adapters.
- Health, readiness, Prometheus metrics, structured logs, and OpenTelemetry spans are implemented.
- k6 scripts and native Go benchmarks are included for smoke, load, stress, and spike validation.
- PostgreSQL persistence, checksum-validated versioned migrations, and a Compose migration container are implemented.
- API keys are role-scoped for tenant, worker, risk, and provider surfaces.
- SPI submission and outbox relay work use persisted claim leases so multiple local workers do not process the same item at the same time.
- Redis-backed distributed rate limiting, a real broker publisher, and ClickHouse remain production-hardening adapters, not MVP blockers.

## Deferred platform work

- Kubernetes manifests are deferred until persistent provider boundaries are implemented.
- Flink is deferred to advanced analytics and does not block the initial payment switch.
- Service mesh is deferred; application-level idempotency, rate limiting, and tracing come first.
- Real provider adapters must enforce external idempotency keys and signed callbacks before production traffic.
