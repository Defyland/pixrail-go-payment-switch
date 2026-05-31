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
- post-persist SPI submission semantics
- request-fingerprint idempotency
- callback-hash settlement dedupe
- executable manual review resolution
- verification report with command output

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
- `db/migrations/0001_pixrail_core.sql`
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
- `internal/spec/repository_spec_test.go`

## Acceptance Criteria Mapping

| Acceptance criterion | Planned implementation |
| --- | --- |
| Required spec-driven docs exist | Create the three files under `docs/spec-driven/`. |
| Product evidence is explicit | Add product problem, personas, use cases, non-goals, roadmap, pricing/plans. |
| Domain model evidence is explicit | Add glossary, bounded contexts, aggregates, invariants, and state machine docs. |
| Case study answers rubric | Add `docs/engineering-case-study.md` with the required table of contents. |
| Data consistency is production-aware | Add PostgreSQL migration and update database design docs. |
| Create is side-effect safe | Persist `accepted` transfer state before SPI submission; record SPI later through an explicit operation. |
| Idempotency is payload-aware | Store request fingerprint and return `409` on mismatched reuse. |
| Review state is operational | Add review decision operation that approves into SPI-pending state or blocks. |
| Callback replay is strict | Store callback hash and reject conflicting terminal callbacks. |
| Readiness is not fake | Add store health interface and readiness failure tests. |
| Outbox is operationally credible | Add relay with publisher ack, failure retry, and tests. |
| Security docs cover abuse and secrets | Add data classification, secrets, and abuse case docs. |
| Scalability and operational cost are explicit | Add required standalone docs. |
| Verification is auditable | Record commands and results in verification report. |

## Verification Commands

```sh
go test ./...
go test -race ./...
PIXRAIL_POSTGRES_TEST_DSN=postgres://pixrail:pixrail@localhost:15432/pixrail?sslmode=disable go test -count=1 -run TestPostgresStoreIntegration -v ./internal/postgres
go vet ./...
npx --yes @redocly/cli lint openapi.yaml
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go test -bench=. -benchmem ./internal/api
go test -run TestCreateTransferLatencyBudget -v ./internal/api
docker build -t pixrail-api:local .
```

## Risks

- Local Docker daemon may be unavailable; if so, Docker build is documented as locally blocked and covered by CI.
- PostgreSQL integration is repeatable through the CI service container and local Compose DSN.
- In-memory adapter remains the local default; production must use durable storage in a real deployment.

## Deferred Work

- Broker-backed outbox publisher.
- Redis-backed distributed rate limiter.
- Real DICT/SPI provider adapters.
- Signed provider callbacks beyond local SPI message and callback-hash validation.
