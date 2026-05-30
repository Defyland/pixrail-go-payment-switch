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
- `internal/api/server.go`
- `internal/api/server_test.go`
- `internal/store/memory.go`
- `internal/store/memory_test.go`
- `internal/outbox/relay.go`
- `internal/outbox/relay_test.go`
- `internal/spec/repository_spec_test.go`

## Acceptance Criteria Mapping

| Acceptance criterion | Planned implementation |
| --- | --- |
| Required spec-driven docs exist | Create the three files under `docs/spec-driven/`. |
| Product evidence is explicit | Add product problem, personas, use cases, non-goals, roadmap, pricing/plans. |
| Domain model evidence is explicit | Add glossary, bounded contexts, aggregates, invariants, and state machine docs. |
| Case study answers rubric | Add `docs/engineering-case-study.md` with the required table of contents. |
| Data consistency is production-aware | Add PostgreSQL migration and update database design docs. |
| Readiness is not fake | Add store health interface and readiness failure tests. |
| Outbox is operationally credible | Add relay with publisher ack, failure retry, and tests. |
| Security docs cover abuse and secrets | Add data classification, secrets, and abuse case docs. |
| Scalability and operational cost are explicit | Add required standalone docs. |
| Verification is auditable | Record commands and results in verification report. |

## Verification Commands

```sh
go test ./...
go test -race ./...
go vet ./...
npx --yes @redocly/cli lint openapi.yaml
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go test -bench=. -benchmem ./internal/api
go test -run TestCreateTransferLatencyBudget -v ./internal/api
docker build -t pixrail-api:local .
```

## Risks

- Local Docker daemon may be unavailable; if so, Docker build is documented as locally blocked and covered by CI.
- PostgreSQL migration can be validated syntactically and by review without a local database unless a database is available.
- In-memory adapter remains the local default; production must use durable storage in a real deployment.

## Deferred Work

- Real PostgreSQL integration tests using Testcontainers or Compose.
- Broker-backed outbox publisher.
- Redis-backed distributed rate limiter.
- Real DICT/SPI provider adapters.
- Signed provider callbacks.
