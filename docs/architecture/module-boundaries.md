# Module Boundaries

PixRail is organized by payment-switch responsibility, not by MVC layers. The important question for every package is: does it trigger a use case, own application orchestration, own domain rules, define a port, or implement an adapter?

| Module | Architectural role | Responsibility | Must not own |
| --- | --- | --- | --- |
| `cmd/pixrail-api` | process adapter | boot HTTP API, logging, graceful shutdown | payment rules |
| `cmd/pixrail-worker` | process adapter | poll accepted transfers through the SPI use case | direct SQL or SPI rules |
| `cmd/pixrail-migrate` | process adapter | run versioned PostgreSQL migrations | business decisions |
| `internal/api` | primary adapter | HTTP routing, auth, error envelopes, request observability, DTO mapping | domain decisions, SQL, DICT/SPI/fraud calls |
| `internal/app` | composition root | choose memory/PostgreSQL and local provider adapters from config | payment state transitions |
| `internal/switcher` | application use cases and ports | payment switch orchestration, idempotency, transaction intent, event emission, port definitions | HTTP, database, provider, cache, broker details |
| `internal/rail` | domain | transfer/payment routing language, validation, fingerprints, state machine invariants, audit evidence shape | adapters, SQL, HTTP, metrics |
| `internal/events` | domain contract | versioned event envelope and outbox record shape | transport-specific broker clients |
| `internal/dict` | secondary adapter | local participant profile/DICT lookup simulation | fraud policy or transfer state |
| `internal/fraud` | secondary adapter/domain service implementation | rules-based risk scoring behind a use case port | settlement callback state |
| `internal/spi` | secondary adapter | local SPI message creation | ledger postings or transfer persistence |
| `internal/store` | secondary adapter | local in-memory persistence fake | SQL concerns or business rules |
| `internal/postgres` | secondary adapter | durable PostgreSQL persistence, row mapping, transactional writes | business rules not already exposed by `rail` state transitions |
| `internal/ratelimit` | adapter/support module | rate-limit algorithms and process-local limiter implementation | payment status changes |
| `internal/codec` | adapter/support module | event and Redis-like cache payload codecs | payment decisions |
| `internal/messaging` | application/adapter boundary | outbox relay use case and publisher port | transfer decisions |
| `internal/observability` | platform adapter | metrics, traces, runtime visibility | domain state mutation |

## Payment Switch Boundary

`internal/switcher` is the payment switch use case module. It defines output ports for store, participant resolution, fraud scoring, SPI submission, and rate limiting. It emits versioned events and asks adapters to persist or perform work. It does not import concrete adapters.

## Ports

The central ports are:

- `switcher.Store`
- `switcher.ParticipantResolver`
- `switcher.FraudScorer`
- `switcher.SPIClient`
- `switcher.RateLimiter`
- `messaging.OutboxStore`
- `messaging.Publisher`

Adapters implement these ports implicitly. Fakes used in tests satisfy the same contracts.

## DTO And Model Boundaries

- HTTP DTOs live in `internal/api` and are mapped into domain/application request types.
- PostgreSQL rows live in `internal/postgres` and are mapped into `rail.Transfer`.
- Cache/binary payload models live in `internal/codec` and do not enter the domain.
- `rail.Transfer`, `rail.CreateTransferRequest`, `rail.SettlementCallback`, and `rail.AuditRecord` are payment-switch domain/application types, not SQL row structs.

## Enforcement

`internal/spec/architecture_spec_test.go` enforces the dependency rule. If a future change makes `internal/switcher` import `internal/postgres`, `internal/store`, `internal/dict`, `internal/fraud`, `internal/spi`, or `internal/ratelimit`, `go test ./internal/spec` fails.
