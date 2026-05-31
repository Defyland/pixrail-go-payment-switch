# PixRail

PixRail is a Go payment switch for Pix-like instant transfers. It models the hot path between a fintech product and the payment network: authenticate the tenant, accept a transfer intent, resolve the receiver key, rate-limit the route, score fraud risk, create a SPI-style message, record settlement callbacks, and publish payment-rail events.

## What is this product?

PixRail is a real-time payment rail API for platforms that need a low-latency Pix routing layer without mixing payment-network decisions with ledger ownership. It is intentionally a payment switch, not the source of truth for balances.

## Problem it solves

Fintech teams often need to prove Pix transfer intake, DICT lookup behavior, fraud decisioning, idempotency, SPI callbacks, observability, and event contracts before integrating with real banking providers. PixRail gives that workflow a runnable backend and documents the operational controls a senior team would expect.

## Target users

- platform engineers building payment products
- fintech backend teams validating Pix hot-path architecture
- SRE and risk teams reviewing rate limits, audit logs, and failure handling
- portfolio reviewers looking for production-minded Go backend evidence

## Main features

- role-scoped API key authentication with tenant isolation
- `POST /v1/pix/transfers` idempotent transfer creation
- DICT-like receiver key resolution with timeout and not-found simulation
- token-bucket rate limiting for tenant/account and DICT lookup pressure
- rules-based antifraud decision log with approve, review, and block outcomes
- SPI-style message creation after the transfer is durably accepted
- manual review resolution for review-threshold transfers
- idempotent settlement callback handling with SPI message and callback-hash guards
- CloudEvents-like outbox records for downstream systems
- versioned PostgreSQL migrations plus leased SPI/outbox worker claims
- structured JSON logs, request ID, correlation ID, Prometheus metrics, and OpenTelemetry spans
- health and readiness probes

## Architecture overview

PixRail is a modular monolith with explicit ports for DICT, antifraud, SPI, storage, rate limiting, and event publishing. The local runtime can use in-memory adapters so the product is runnable without cloud dependencies; the production path uses PostgreSQL for transfer, audit, and outbox durability, and keeps Redis/broker adapters as the next deployment hardening step.

```text
Tenant API -> HTTP API -> Payment Switch -> DICT Resolver
                                  |-> Fraud Engine
                                  |-> SPI Simulator
                                  |-> Transfer Store + Audit Log + Outbox
                                  |-> Metrics/Logs/Traces
```

The switch owns payment-rail state only. Settlement, ledger entries, balances, reconciliation, and financial reporting remain outside this service.

## Tech stack

- Go 1.25
- standard `net/http` router
- OpenTelemetry API and SDK
- structured logs through `log/slog`
- Prometheus text metrics
- PostgreSQL adapter and migration runner
- long-running SPI submission worker
- Docker and Compose
- k6 benchmark scripts
- GitHub Actions CI

## Domain model

- `Transfer`: tenant-scoped Pix transfer intent, request fingerprint, fraud decision, SPI identifiers, and settlement status
- `DictEntry`: resolved receiver identity, bank ISPB, account hash, and risk signal
- `FraudDecision`: score, triggered rules, decision reason, and resulting status
- `SPIMessage`: simulated payment-network message with end-to-end ID
- `OutboxRecord`: durable event envelope for downstream consumers
- `AuditRecord`: immutable operational evidence for decisions and callbacks

Detailed domain evidence lives in [docs/domain](docs/domain), and the senior case-study narrative lives in [docs/engineering-case-study.md](docs/engineering-case-study.md).

## API documentation

The HTTP contract is versioned under `/v1` and documented in [openapi.yaml](openapi.yaml). Examples live in [docs/api/request-response-examples.md](docs/api/request-response-examples.md), and the shared error envelope is documented in [docs/api/error-format.md](docs/api/error-format.md).

Authentication accepts either `Authorization: Bearer <api-key>` or `X-API-Key: <api-key>`. API keys are role-scoped: `tenant` keys create/read transfers, `worker` keys submit accepted transfers to SPI, `risk` keys resolve manual reviews, and `provider` keys send settlement callbacks. Local development seeds `dev-secret`, `worker-secret`, `risk-secret`, and `provider-secret` for tenant `tenant_demo`; production requires `PIXRAIL_API_KEYS`.

## Async or event architecture

Every accepted transfer writes events to an outbox in the same logical transaction as the transfer state. Create does not call SPI before this durable write; it records `accepted` plus `spi_submission_requested`, then SPI submission records `approved` with SPI identifiers. Events use the envelope documented in [docs/events/README.md](docs/events/README.md) and include `event_id`, `event_type`, `schema_version`, `occurred_at`, `producer`, `tenant_id`, `account_id`, `pix_transfer_id`, and `correlation_id`.

The messaging topology defines a payment rail exchange, routing key, consumer queue, retry queue, dead-letter exchange, dead-letter queue, idempotency header, and correlation header in code and documentation. The relay claims pending outbox rows with a lease, publishes them through a publisher interface, marks acknowledgements only when the claim token matches, and schedules failed publishes for retry. Consumers must deduplicate by `event_id` and preserve account-level ordering.

## Database design

The local default uses an in-memory repository so tests and simple demos have no external dependency. Production mode requires `PIXRAIL_STORE_DRIVER=postgres` and `PIXRAIL_DATABASE_URL`. PostgreSQL migrations are versioned under [db/migrations](db/migrations), the checksum-validating migration runner is [cmd/pixrail-migrate](cmd/pixrail-migrate), the long-running SPI worker is [cmd/pixrail-worker](cmd/pixrail-worker), and the adapter is implemented under [internal/postgres](internal/postgres).

Transaction boundary: transfer state, request fingerprint, decision audit, and outbox inserts are committed together. SPI submission happens only after a persisted accepted transfer exists and a worker claim has been stored. Settlement callbacks are guarded by SPI message ID, callback hash, and terminal-state checks.

## Testing strategy

The suite uses Go `testing` and `httptest`:

- unit tests for validation, rate limiting, DICT simulation, fraud rules, and messaging topology
- API/request tests for auth, validation, lifecycle, metrics, and rate-limit failure
- store tests for idempotency and tenant isolation
- service tests for accepted, SPI-submitted, review, blocked, idempotent replay, and settlement flows
- repository spec tests for required docs, OpenAPI, benchmark artifacts, and event schema envelope coverage
- native benchmark for transfer creation hot path

## Performance benchmarks

Benchmark methodology and local results are in [docs/benchmarks/methodology.md](docs/benchmarks/methodology.md), [benchmarks/baseline.md](benchmarks/baseline.md), and [benchmarks/results/2026-05-30-local-baseline.md](benchmarks/results/2026-05-30-local-baseline.md). k6 scripts cover smoke, load, stress, and spike profiles under [benchmarks/k6](benchmarks/k6).

## Observability

PixRail exposes:

- `GET /healthz`
- `GET /readyz`, backed by store health
- `GET /metrics`
- JSON request logs with request and correlation IDs
- OpenTelemetry spans around HTTP routes; stdout exporting is controlled by `PIXRAIL_TRACING_EXPORTER`
- Prometheus counters and latency quantiles
- Grafana dashboard definition in [observability/grafana/pixrail-overview-dashboard.json](observability/grafana/pixrail-overview-dashboard.json)

## Security considerations

Security coverage is documented in [docs/security/threat-model.md](docs/security/threat-model.md), [docs/security/authorization-matrix.md](docs/security/authorization-matrix.md), [docs/security/abuse-cases.md](docs/security/abuse-cases.md), [docs/security/data-classification.md](docs/security/data-classification.md), and [docs/security/secrets.md](docs/security/secrets.md). The implementation covers role-scoped API key authentication, tenant isolation, idempotency, rate limiting, input validation, audit logging, correlation IDs, production config guards, and environment-based secret configuration.

## Trade-offs and decisions

- PixRail is a payment rail, not the ledger; see [ADR 0001](docs/adr/0001-payment-rail-boundary-before-financial-core.md).
- A modular monolith is used before microservices because the hot path needs clear local transaction boundaries first.
- In-memory adapters are accepted for local development; PostgreSQL is the durable store path, while Redis and broker adapters are the next production-hardening slice.
- Full Event Sourcing, Kubernetes, service mesh, CDC, and data-lake analytics are deferred until provider integrations and persistence are real.

Spec-driven readiness evidence is maintained in [docs/spec-driven](docs/spec-driven). Scalability and operational-cost analysis live in [docs/scalability.md](docs/scalability.md) and [docs/operational-cost.md](docs/operational-cost.md).

## How to run locally

```sh
go run ./cmd/pixrail-api
```

The local API listens on `:8080` and seeds four role-scoped demo keys: `dev-secret` (`tenant`), `worker-secret` (`worker`), `risk-secret` (`risk`), and `provider-secret` (`provider`).

```sh
curl -s http://localhost:8080/healthz
curl -s -X POST http://localhost:8080/v1/pix/transfers \
  -H 'Authorization: Bearer dev-secret' \
  -H 'Idempotency-Key: demo-1' \
  -H 'Content-Type: application/json' \
  -d '{"account_id":"acct_123","amount_cents":12345,"currency":"BRL","receiver_key":"receiver@example.com","receiver_key_type":"EMAIL"}'
```

The create response is `accepted` and intentionally has no SPI identifiers yet. To simulate the post-persist SPI worker locally:

```sh
curl -s -X POST http://localhost:8080/v1/pix/transfers/{id}/spi-submissions \
  -H 'Authorization: Bearer worker-secret'
```

Compose is available for production-like process wiring:

```sh
docker compose -f compose.yaml up --build
```

The Compose path starts PostgreSQL, applies migrations, starts the API, and starts `pixrail-worker`. The worker polls accepted transfers with `PIXRAIL_WORKER_BATCH_SIZE` and `PIXRAIL_WORKER_INTERVAL`, then uses the same claim-protected `SubmitPendingSPI` path exposed for local API-triggered simulation.

To run against PostgreSQL, apply the versioned migrations with `pixrail-migrate` and start with:

```sh
PIXRAIL_STORE_DRIVER=postgres \
PIXRAIL_HTTP_ADDR=:18080 \
PIXRAIL_DATABASE_URL=postgres://pixrail:pixrail@localhost:15432/pixrail?sslmode=disable \
PIXRAIL_API_KEYS=tenant_demo:dev-secret:tenant,tenant_demo:worker-secret:worker,tenant_demo:risk-secret:risk,tenant_demo:provider-secret:provider \
go run ./cmd/pixrail-api
```

To run the worker manually against the same PostgreSQL database:

```sh
PIXRAIL_STORE_DRIVER=postgres \
PIXRAIL_DATABASE_URL=postgres://pixrail:pixrail@localhost:15432/pixrail?sslmode=disable \
PIXRAIL_API_KEYS=tenant_demo:dev-secret:tenant,tenant_demo:worker-secret:worker,tenant_demo:risk-secret:risk,tenant_demo:provider-secret:provider \
PIXRAIL_WORKER_BATCH_SIZE=100 \
PIXRAIL_WORKER_INTERVAL=1s \
go run ./cmd/pixrail-worker
```

The Compose path starts PostgreSQL, applies migrations with `pixrail-migrate`, then boots the API and worker with `PIXRAIL_STORE_DRIVER=postgres`.

To apply the migrations manually:

```sh
PIXRAIL_DATABASE_URL='postgres://pixrail:pixrail@localhost:15432/pixrail?sslmode=disable' \
  go run ./cmd/pixrail-migrate
```

## How to run tests

```sh
go test ./...
go test -bench=. ./internal/api
gofmt -w cmd internal
go vet ./...
```

Optional PostgreSQL integration test:

```sh
PIXRAIL_POSTGRES_TEST_DSN='postgres://pixrail:pixrail@localhost:15432/pixrail?sslmode=disable' \
  go test ./internal/postgres -run Integration
```

## Failure scenarios

- missing or invalid API key returns `401`
- valid API key without the endpoint role returns `403`
- missing idempotency key or invalid payload returns `400`
- tenant/account bucket exhaustion returns `429`
- DICT missing or timeout simulation returns dependency failure
- high-risk receivers are blocked before SPI message creation
- duplicate transfer requests replay the original response without new events
- duplicate transfer requests with a different payload return `409`
- duplicate terminal SPI callbacks replay only when the callback hash matches
- active SPI or outbox leases prevent concurrent workers from duplicating the same local work item
- wrong tenant cannot read another tenant transfer

## Roadmap

- add Redis-backed distributed rate limiting
- add broker-backed publisher for the existing outbox relay
- add provider adapters for real DICT, antifraud, and SPI integrations
- add signed internal callbacks and processed-message inbox
- add ClickHouse projection for risk and payment analytics
- run k6 benchmarks against a Compose environment and publish measured artifacts per release
