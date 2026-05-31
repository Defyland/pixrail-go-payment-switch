# Implementation Plan

## Scope

Apply the new senior spec-driven standards to PixRail while keeping work scoped to `pixrail-go-payment-switch/`. The pass focuses on evidence that materially improves senior/tech-lead evaluation:

- required spec-driven documentation
- product and domain documentation
- engineering case study
- scalability and operational cost analysis
- readiness tied to dependency health
- outbox relay/retry semantics
- PostgreSQL migration evidence
- checksum-validated versioned migrations
- post-persist SPI submission semantics
- SPI and outbox worker claim leases
- long-running SPI submission worker process
- role-scoped API keys for tenant, worker, risk, and provider actions
- HMAC-signed provider callbacks
- request-fingerprint idempotency
- callback-hash settlement dedupe
- executable manual review resolution
- verification report with command output
- serialization benchmarks for JSON, Protobuf wire, MsgPack, and CBOR
- Redis-like participant profile cache codecs
- executable rate-limiting strategies with recommendations
- Go runtime/container hardening evidence
- trace-buffering guidance without pretending local Kafka/Redpanda exists
- explicit Modular Monolith, Hexagonal/Ports & Adapters, and pragmatic DDD boundaries
- dependency-rule tests proving PixRail is not MVC renamed

## Files to Create or Update

- `docs/spec-driven/senior-readiness-spec.md`
- `docs/spec-driven/implementation-plan.md`
- `docs/spec-driven/verification-report.md`
- `docs/engineering-case-study.md`
- `docs/product/*.md`
- `docs/domain/*.md`
- `docs/scalability.md`
- `docs/operational-cost.md`
- `docs/security/*.md`
- `docs/architecture/*.md`
- `README.md`
- `db/migrations/*.sql`
- `cmd/pixrail-migrate/main.go`
- `cmd/pixrail-worker/main.go`
- `openapi.yaml`
- `internal/api/server.go`
- `internal/api/server_test.go`
- `internal/rail/model.go`
- `internal/fraud/engine.go`
- `internal/switcher/service.go`
- `internal/switcher/service_test.go`
- `internal/postgres/store.go`
- `internal/store/memory.go`
- `internal/store/memory_test.go`
- `internal/messaging/relay.go`
- `internal/messaging/relay_test.go`
- `internal/codec/*.go`
- `internal/ratelimit/*.go`
- `internal/observability/metrics.go`
- `internal/app/pprof.go`
- `internal/spec/architecture_spec_test.go`
- `internal/spec/repository_spec_test.go`
- `docs/architecture/ports-and-adapters.md`
- `docs/architecture/go-architecture.md`
- `docs/architecture/dependency-rule.md`
- `docs/architecture/testing-strategy.md`
- `docs/benchmarks/serialization.md`
- `docs/rate-limiting.md`
- `docs/runtime/gomaxprocs-kubernetes.md`
- `docs/observability/trace-buffering.md`
- `docs/production-readiness.md`

## Acceptance Criteria Mapping

| Acceptance criterion | Planned implementation |
| --- | --- |
| Required spec-driven docs exist | Create the three files under `docs/spec-driven/`. |
| Product evidence is explicit | Add product problem, personas, use cases, non-goals, roadmap, pricing/plans. |
| Domain model evidence is explicit | Add glossary, bounded contexts, aggregates, invariants, and state machine docs. |
| Case study answers rubric | Add `docs/engineering-case-study.md` with the required table of contents. |
| Data consistency is production-aware | Add PostgreSQL migration and update database design docs. |
| Architecture is not MVC renamed | Document ports/adapters, Go architecture, module boundaries, and dependency rule. |
| Use cases depend on ports | Declare ports in `internal/switcher` and wire concrete adapters only in `internal/app`. |
| Domain owns state transitions | Move SPI/review/settlement transition rules into `internal/rail` and test them without persistence. |
| Dependency rule is executable | Add spec tests that parse production imports and fail on boundary violations. |
| Create is side-effect safe | Persist `accepted` transfer state before SPI submission; record SPI later through an explicit operation. |
| External work is claim-protected | Add SPI and outbox leases with claim-token checked updates. |
| Pending SPI work has an operational process | Add `cmd/pixrail-worker` and wire it into Compose. |
| Idempotency is payload-aware | Store request fingerprint and return `409` on mismatched reuse. |
| Operational surfaces are role-scoped | Require separate API key roles for tenant API, SPI worker, risk review, and provider callback actions. |
| Provider callbacks are tamper-evident | Require timestamped HMAC signatures on SPI callback requests. |
| Review state is operational | Add review decision operation that approves into SPI-pending state or blocks. |
| Callback replay is strict | Store callback hash and reject conflicting terminal callbacks. |
| Readiness is not fake | Add store health interface and readiness failure tests. |
| Outbox is operationally credible | Add relay with publisher ack, failure retry, and tests. |
| Security docs cover abuse and secrets | Add data classification, secrets, and abuse case docs. |
| Serialization choices are evidence-backed | Add codec package, codec tests, benchmarks, and benchmark documentation. |
| Redis-like cache state has concrete binary contracts | Add MsgPack and CBOR participant-profile cache codecs with malformed-payload tests. |
| Rate limiting is not a placeholder | Implement fixed window, sliding window, leaky bucket, token bucket tests/benchmarks, and endpoint recommendations. |
| Runtime hardening is explicit | Add startup runtime logs, runtime metrics, pprof opt-in, PostgreSQL pool config, and container CPU docs. |
| Trace buffering is honest | Document Redpanda/Kafka trace buffering as production option outside local scope. |
| Scalability and operational cost are explicit | Add required standalone docs. |
| Verification is auditable | Record commands and results in verification report. |

## Verification Commands

```sh
go test ./...
go test ./internal/spec
go test -race ./...
PIXRAIL_POSTGRES_TEST_DSN=postgres://pixrail:pixrail@localhost:15432/pixrail?sslmode=disable go test -count=1 -run TestPostgresStoreIntegration -v ./internal/postgres
go vet ./...
npx --yes @redocly/cli lint openapi.yaml
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go test -bench=. -benchmem ./internal/api
go test -bench=. -benchmem ./internal/codec
go test -bench=. -benchmem ./internal/ratelimit
go test -run TestCreateTransferLatencyBudget -v ./internal/api
docker build -t pixrail-api:local .
```

## Risks

- Local Docker daemon may be unavailable; if so, Docker build is documented as locally blocked and covered by CI.
- PostgreSQL integration is repeatable through the CI service container and local Compose DSN.
- In-memory adapter remains the local default; production must use durable storage in a real deployment.
- Benchmark numbers vary by host and Go version; the committed docs record one local run and keep commands executable.

## Deferred Work

- Broker-backed outbox publisher.
- Redis-backed distributed rate limiter.
- Real DICT/SPI provider adapters.
