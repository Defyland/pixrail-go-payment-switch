# Senior Readiness Spec

This spec applies `specs/general-project-spec.md`, `specs/senior-engineering-rubric.md`, and `specs/spec-driven-senior-quality.md` to PixRail.

## Product Bar

PixRail must read as a believable instant-payment switch product: it names who uses it, the Pix hot-path problem it solves, the operational workflow, non-goals, and business value. It must avoid pretending to be a ledger or bank core.

## Domain Bar

The domain evidence must define payment-rail language, bounded contexts, aggregates, invariants, and transfer state machines. Domain rules in docs must map to code and tests: idempotency, tenant isolation, DICT lookup behavior, fraud decisions, SPI callbacks, and terminal state handling.

## Architecture Bar

The architecture must justify a modular monolith with explicit ports for DICT, fraud, SPI, storage, rate limiting, observability, and outbox publishing. It must include deployment view, module boundaries, sequence flows, and rejected alternatives.

## API Bar

The API must provide versioned endpoints, OpenAPI, API key authentication, idempotency rules, standardized errors, request/response examples, validation failures, and authorization failures.

## Data and Consistency Bar

PixRail must document and, where code exists, enforce transactional boundaries for transfer state, audit evidence, and outbox events. The project must document PostgreSQL constraints and keep the in-memory adapter honest as a local/testing adapter.

## Security Bar

Security evidence must cover API keys, tenant isolation, BOLA, abuse cases, rate-limit bypass, idempotency abuse, audit logging, secrets, and data classification. High-risk controls must have tests when practical.

## Observability Bar

The service must expose health, readiness, structured logs, request ID, correlation ID, domain metrics, Prometheus output, OpenTelemetry traces, dashboard JSON, alerts, and runbooks. Readiness must reflect dependency health, not just process liveness.

## Performance Bar

Benchmarks must include smoke/load/stress/spike k6 profiles plus local measured p50, p95, p99, throughput, error rate, CPU/memory notes, bottlenecks, and next optimization.

## Scalability Bar

The docs must identify hot paths, write-heavy operations, fastest-growing tables, outbox buildup, hot partitions, horizontal scaling points, sharding candidates, async candidates, and non-eventual flows.

## Operational Cost Bar

The docs must discuss infrastructure cost, debugging cost, deploy cost, backup/retention, monitoring burden, vendor lock-in risk, and simpler alternatives rejected.

## Maintainability Bar

The repository must make extension points obvious: adding a fraud rule, replacing DICT, replacing SPI, moving from memory to PostgreSQL, and adding a real broker publisher.

## Readability Bar

Code, tests, and docs must use PixRail domain language instead of generic processing language. Claims must link to evidence.

## Test and CI Bar

The project must run Go tests, race tests, vet, OpenAPI validation, security scan, coverage, Docker build, Compose smoke, PostgreSQL integration, and performance checks. CI covers the repeatable quality gates, while local Compose evidence proves the durable runtime path.

## Evidence Matrix

| Criterion | Evidence | Status | Notes |
| --- | --- | --- | --- |
| Product problem and users are explicit | `README.md`, `docs/product/problem.md`, `docs/product/personas.md` | Done | Product is payment switch, not ledger. |
| Domain language is defined | `docs/domain/glossary.md`, `docs/domain/bounded-contexts.md` | Done | Terms align with code packages. |
| Aggregates and invariants are explicit | `docs/domain/aggregates.md`, `docs/domain/invariants.md`, `internal/switcher/service_test.go` | Done | Invariants include idempotency, tenant isolation, terminal callbacks. |
| State machine is documented and tested | `docs/domain/state-machines.md`, `internal/switcher/service_test.go`, `internal/store/memory_test.go` | Done | Transfer states map to `rail.TransferStatus`. |
| API contract is versioned and validated | `openapi.yaml`, `docs/api/request-response-examples.md`, Redocly lint | Done | Includes auth and failure examples. |
| Data consistency boundaries are documented | `docs/architecture/database-design.md`, `db/migrations/0001_pixrail_core.sql` | Done | PostgreSQL schema is migration evidence; local runtime can still use memory. |
| Readiness reflects dependency health | `internal/api/server.go`, `internal/api/server_test.go` | Done | Health remains liveness; readiness checks store health. |
| Outbox relay has retry semantics | `internal/messaging/relay.go`, `internal/messaging/relay_test.go`, `internal/store/memory.go` | Done | Relay handles publish ack and retry evidence. |
| Security model covers BOLA and secrets | `docs/security/threat-model.md`, `docs/security/authorization-matrix.md`, `docs/security/abuse-cases.md`, `docs/security/secrets.md` | Done | Tests cover tenant isolation and auth. |
| Observability has domain metrics and runbooks | `internal/observability/metrics.go`, `observability/grafana/pixrail-overview-dashboard.json`, `docs/observability/overview.md`, `docs/runbooks/` | Done | Domain decision/outbox metrics are present. |
| Performance evidence is measured | `benchmarks/results/2026-05-30-local-baseline.md`, `internal/api/server_test.go`, `benchmarks/k6/` | Done | Native p50/p95/p99 plus k6 smoke/load/stress/spike output recorded. |
| Scalability and cost are explicit | `docs/scalability.md`, `docs/operational-cost.md` | Done | Names bottlenecks and accepted cost. |
| CI covers quality gates | `.github/workflows/ci.yml` | Done | Includes format, tests, security, OpenAPI, Docker. |
| Docker and Compose validated locally | `Dockerfile`, `compose.yaml`, `docker-compose.yml`, `docs/spec-driven/verification-report.md` | Done | Local Docker build, PostgreSQL Compose, migration runner, API, Prometheus, smoke, and k6 all passed. |
| Real provider certification | external Pix/DICT/SPI providers | Planned | Out of scope for portfolio MVP. |

## Out of Scope

- Real Pix/SPI provider certification.
- Full Event Sourcing.
- Multi-region active-active payment processing.
- Kubernetes and service mesh manifests.
- ClickHouse analytics implementation.
- Production secret rotation automation.
