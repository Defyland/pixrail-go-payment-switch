# Verification Report

## Summary

PixRail was updated against `specs/general-project-spec.md`, `specs/senior-engineering-rubric.md`, and `specs/spec-driven-senior-quality.md`.

The repository now has the required spec-driven documents, product and domain evidence, engineering case study, scalability and operational-cost analysis, expanded architecture views, checksum-validated PostgreSQL migrations, dependency-backed readiness, claim-protected outbox relay behavior, claim-protected SPI submission, role-scoped API keys, request-fingerprint idempotency, callback-hash settlement replay, executable review resolution, configurable trace exporting, k6 evidence, and updated repository conformance tests.

## Commands Run

| Command | Result | Evidence |
| --- | --- | --- |
| `go test ./...` | Passed | All packages green, including `internal/postgres`, `internal/messaging`, `internal/observability`, and `internal/spec`. |
| `go test ./...` after worker-lease, role-auth, and docs hardening | Passed | All packages green after `0003_worker_leases.sql`, SPI/outbox claims, endpoint role checks, and repository spec updates. |
| `go test -race ./...` | Passed | Race-enabled suite passed across all packages. |
| `go vet ./...` | Passed | No vet findings. |
| `npx --yes @redocly/cli lint openapi.yaml` | Passed | OpenAPI validated in 19 ms with no warnings. |
| `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | Passed | `No vulnerabilities found.` |
| `docker compose -f compose.yaml config` | Passed | Compose rendered successfully after role-scoped `PIXRAIL_API_KEYS`; generated config had 88 lines. |
| `go test -bench=. -benchmem ./internal/api` | Passed | `BenchmarkCreateTransfer-10 48933 31595 ns/op 27823 B/op 221 allocs/op`. |
| `go test -run TestCreateTransferLatencyBudget -v ./internal/api` | Passed | p50 `17.459us`, p95 `25.833us`, p99 `52.375us`, throughput `40658 rps`, error rate `0.00%`. |
| `docker build -t pixrail-api:local .` | Passed | Multi-stage API image built successfully. |
| `docker compose -f compose.yaml up -d --build` | Passed | PostgreSQL 17 healthy on host `15432`, API up on `18080`, Prometheus up on `19090`, migration container completed. |
| `curl -i http://127.0.0.1:18080/readyz` | Passed | Store-backed readiness returned `HTTP 200` with `{"dependency":"store","status":"ready"}`. |
| PostgreSQL HTTP smoke | Passed | `POST /v1/pix/transfers` returned `accepted` with empty SPI IDs; `POST /spi-submissions` returned `approved` with SPI ID; settlement callback returned `settled`; wrong SPI callback returned `409`. |
| `PIXRAIL_POSTGRES_TEST_DSN=postgres://pixrail:pixrail@localhost:15432/pixrail?sslmode=disable go test -count=1 -run TestPostgresStoreIntegration -v ./internal/postgres` | Passed | Live PostgreSQL migration/store integration passed, including pending SPI, SPI submission, settlement, and callback-hash replay. |
| `BASE_URL=http://127.0.0.1:18080 PIXRAIL_API_KEY=dev-secret k6 run benchmarks/k6/smoke.js` | Passed | 5/5 checks, 0% failures, p95 `7.39 ms`. |

## Passing Criteria

- Required `docs/spec-driven/` files exist and contain acceptance criteria, plan, and verification.
- README points to product, domain, spec-driven, case study, scalability, and operational-cost evidence.
- Product docs cover problem, personas, use cases, non-goals, roadmap, and plan framing.
- Domain docs cover glossary, bounded contexts, aggregates, invariants, and state machines.
- Architecture docs include overview, C4 context, C4 container, module boundaries, sequence diagrams, deployment view, and ADRs.
- PostgreSQL migration exists with unique idempotency, event, and SPI constraints.
- PostgreSQL migrations are versioned and checksum-validated through `schema_migrations`.
- Request fingerprint is stored and mismatched idempotency replay returns conflict.
- Create persists `accepted` state before SPI submission; SPI identifiers are recorded through explicit post-persist operation.
- SPI submission claims the accepted transfer before the SPI client call and checks the claim token before approval persistence.
- Outbox relay claims records before publish and checks the claim token before publish/failure updates.
- API keys are role-scoped for tenant, worker, risk, and provider actions.
- Review state has an executable approve/block path.
- Terminal settlement replay is guarded by callback hash and SPI message ID.
- Production config rejects memory storage and requires PostgreSQL DSN.
- Readiness checks store health.
- Outbox relay publishes, marks acknowledgements, and schedules retries with error evidence.
- Security docs cover threat model, authorization matrix, data classification, secrets, and abuse cases.
- Benchmarks include measured p50, p95, p99, throughput, error rate, memory, and allocation evidence.
- Docker and Compose were validated locally with a live PostgreSQL path.
- k6 smoke was validated against the Compose PostgreSQL runtime after the consistency changes. Prior load/stress/spike artifacts remain historical benchmark evidence and should be rerun before a release tag.

## Partial Criteria

- Broker-backed publisher is still an adapter milestone; relay semantics are implemented with an interface and in-memory publisher.
- Redis-backed distributed rate limiting is still a scale-out milestone; the current limiter is bounded to one process.

## Failed or Blocked Criteria

- None in this local verification pass.

## Remaining Risk

- Add broker-backed publisher integration and DLQ replay tooling.
- Add Redis-backed distributed rate limiting before horizontal API scaling.
- Add signed SPI callback verification before external provider integration.
- Add long-running worker process around `SubmitPendingSPI` when moving beyond local/API-triggered simulation.
- Add real provider idempotency keys and signed callback verification before external SPI traffic.
