# Verification Report

## Summary

PixRail was updated against `specs/general-project-spec.md`, `specs/senior-engineering-rubric.md`, and `specs/spec-driven-senior-quality.md`.

The repository now has the required spec-driven documents, product and domain evidence, engineering case study, scalability and operational-cost analysis, expanded architecture views, explicit Go Modular Monolith/Hexagonal documentation, executable dependency-rule tests, checksum-validated PostgreSQL migrations, dependency-backed readiness, claim-protected outbox relay behavior, claim-protected SPI submission, a long-running SPI worker process, role-scoped API keys, HMAC-signed provider callbacks, strict event payload schemas, request-fingerprint idempotency, callback-hash settlement replay, executable review resolution, configurable trace exporting, runtime metrics, optional pprof, explicit PostgreSQL pool configuration, serialization benchmarks, Redis-like cache codecs, rate-limiting strategy benchmarks, k6 evidence, and updated repository conformance tests.

## Commands Run

| Command | Result | Evidence |
| --- | --- | --- |
| `go test ./...` | Passed | All packages green, including `internal/postgres`, `internal/messaging`, `internal/observability`, and `internal/spec`. |
| `go test ./internal/spec` | Passed | Repository artifact checks and architecture dependency-rule checks passed. |
| `go test ./internal/rail ./internal/switcher ./internal/store ./internal/postgres` | Passed | Domain state machine, use case, memory adapter, and PostgreSQL adapter tests passed after boundary hardening. |
| `go test ./...` after worker, event-schema, and docs hardening | Passed | All packages green after `cmd/pixrail-worker`, strict event payload schemas, and repository spec updates. |
| `go test -race ./...` | Passed | Race-enabled suite passed across all packages. |
| `go vet ./...` | Passed | No vet findings. |
| `npx --yes @redocly/cli lint openapi.yaml` | Passed | OpenAPI validated in 20 ms with no warnings. |
| `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | Passed | `No vulnerabilities found.` |
| `docker compose -f compose.yaml config` | Passed | Compose rendered successfully after API, worker, and callback-secret wiring; generated config had 112 lines. |
| `go test -bench=. -benchmem ./internal/api` | Passed | `BenchmarkCreateTransfer-10 43267 27636 ns/op 27998 B/op 221 allocs/op`. |
| `go test -run TestCreateTransferLatencyBudget -v ./internal/api` | Passed | p50 `18.542us`, p95 `56.833us`, p99 `203.292us`, throughput `30663 rps`, error rate `0.00%`. |
| `go test -bench=. -benchmem ./internal/codec` | Passed | JSON, Protobuf wire, MsgPack, CBOR, and participant-profile cache codecs benchmarked; results in `docs/benchmarks/serialization.md`. |
| `go test -bench=. -benchmem ./internal/ratelimit` | Passed | Token bucket, fixed window, sliding window, and leaky bucket benchmarked; results in `docs/rate-limiting.md`. |
| `docker build -t pixrail-api:local .` | Passed | Multi-stage API image built successfully. |
| `docker compose -f compose.yaml up -d --build` | Passed | PostgreSQL 17 healthy on host `15432`, API up on `18080`, Prometheus up on `19090`, migration container completed. |
| `curl -i http://127.0.0.1:18080/readyz` | Passed | Store-backed readiness returned `HTTP 200` with `{"dependency":"store","status":"ready"}`. |
| PostgreSQL HTTP smoke | Passed | `POST /v1/pix/transfers` returned `accepted` with empty SPI IDs; `POST /spi-submissions` returned `approved` with SPI ID; settlement callback returned `settled`; wrong SPI callback returned `409`. |
| `PIXRAIL_POSTGRES_TEST_DSN=postgres://pixrail:pixrail@localhost:15432/pixrail?sslmode=disable go test -count=1 -run TestPostgresStoreIntegration -v ./internal/postgres` | Passed | Live PostgreSQL migration/store integration passed, including pending SPI, SPI submission, settlement, and callback-hash replay. |
| `BASE_URL=http://127.0.0.1:18080 PIXRAIL_API_KEY=dev-secret k6 run benchmarks/k6/smoke.js` | Passed | 5/5 checks, 0% failures, p95 `7.39 ms`. |
| Local memory API plus `k6 run benchmarks/k6/smoke.js` on port `18081` | Passed | 5/5 checks, measured phase p95 `679.8us`, measured error rate `0.00%`; warmup traffic is tagged separately. |

## Passing Criteria

- Required `docs/spec-driven/` files exist and contain acceptance criteria, plan, and verification.
- README points to product, domain, spec-driven, case study, scalability, and operational-cost evidence.
- Product docs cover problem, personas, use cases, non-goals, roadmap, and plan framing.
- Domain docs cover glossary, bounded contexts, aggregates, invariants, and state machines.
- Architecture docs include overview, C4 context, C4 container, ports/adapters, Go architecture, dependency rule, module boundaries, testing strategy, sequence diagrams, deployment view, and ADRs.
- PixRail is explicitly documented and tested as a Go modular monolith with Hexagonal/Ports & Adapters boundaries, not MVC renamed.
- `internal/switcher` declares application ports and imports only domain/event packages.
- `internal/rail` owns transfer validation, fingerprints, audit evidence shape, and payment state transitions without importing infra.
- HTTP DTOs stay inside `internal/api`; database/cache models stay in adapters.
- PostgreSQL migration exists with unique idempotency, event, and SPI constraints.
- PostgreSQL migrations are versioned and checksum-validated through `schema_migrations`.
- Request fingerprint is stored and mismatched idempotency replay returns conflict.
- Create persists `accepted` state before SPI submission; SPI identifiers are recorded through explicit post-persist operation.
- SPI submission claims the accepted transfer before the SPI client call and checks the claim token before approval persistence.
- `cmd/pixrail-worker` continuously drains accepted transfers through `SubmitPendingSPI`.
- Outbox relay claims records before publish and checks the claim token before publish/failure updates.
- Event schemas define strict payload fields, not only a generic envelope.
- API keys are role-scoped for tenant, worker, risk, and provider actions.
- Provider callbacks require timestamped HMAC signatures before settlement processing.
- Review state has an executable approve/block path.
- Terminal settlement replay is guarded by callback hash and SPI message ID.
- Production config rejects memory storage and requires PostgreSQL DSN.
- Readiness checks store health.
- Outbox relay publishes, marks acknowledgements, and schedules retries with error evidence.
- Security docs cover threat model, authorization matrix, data classification, secrets, and abuse cases.
- Benchmarks include measured p50, p95, p99, throughput, error rate, memory, and allocation evidence.
- Serialization benchmarks cover JSON, Protobuf wire, MsgPack, CBOR, payment events, and Redis-like participant profile cache state.
- Rate limiting includes token bucket, fixed window, sliding window, and leaky bucket implementations with tests, benchmarks, and endpoint recommendations.
- Runtime hardening includes startup runtime logs, Prometheus runtime metrics, optional pprof, bounded shutdown, client timeouts, and explicit PostgreSQL pool settings.
- k6 profiles tag warmup traffic separately from measured traffic so cold-start behavior is visible without corrupting steady-state thresholds.
- Docker and Compose were validated locally with a live PostgreSQL path.
- k6 smoke was validated against the Compose PostgreSQL runtime after the consistency changes. Prior load/stress/spike artifacts remain historical benchmark evidence and should be rerun before a release tag.

## Partial Criteria

- Broker-backed publisher is still an adapter milestone; relay semantics are implemented with an interface and in-memory publisher.
- Redis-backed distributed rate limiting is still a scale-out milestone; the current limiter is bounded to one process.
- Generated `.proto` packages are not committed; the local Protobuf wire codec is an executable benchmark contract for PixRail internal event payloads.

## Failed or Blocked Criteria

- None in this local verification pass.

## Remaining Risk

- Add broker-backed publisher integration and DLQ replay tooling.
- Add Redis-backed distributed rate limiting before horizontal API scaling.
- Add real provider idempotency keys before external SPI traffic.
