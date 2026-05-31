# Ports And Adapters

PixRail is a modular monolith with Hexagonal/Ports & Adapters boundaries. It is not MVC renamed: there is no controller-model-repository stack where handlers move DTOs into tables. HTTP is only one adapter. The payment switch use cases own orchestration, and the domain owns transfer state rules.

## Adapter Types

| Type | PixRail modules | Responsibility |
| --- | --- | --- |
| Primary adapters | `internal/api`, `cmd/pixrail-api`, `cmd/pixrail-worker`, `cmd/pixrail-migrate` | Translate external triggers into application use case calls. |
| Application/use cases | `internal/switcher`, `internal/messaging` | Orchestrate idempotency, transactions, ports, event emission, and retry/lease semantics. |
| Domain | `internal/rail`, domain parts of `internal/events` | Define payment language, invariants, transfer state transitions, fingerprints, and event envelopes. |
| Output ports | Interfaces declared by `internal/switcher` and `internal/messaging` | Isolate store, participant lookup, fraud scoring, SPI submission, rate limiting, and outbox publishing. |
| Secondary adapters | `internal/postgres`, `internal/store`, `internal/dict`, `internal/fraud`, `internal/spi`, `internal/ratelimit`, `internal/codec`, `internal/observability` | Implement infrastructure, local fakes, simulations, codecs, metrics, and platform details. |

## Primary Flow

```text
HTTP/worker/CLI adapter
  -> switcher use case
     -> rail domain invariants and state transitions
     -> output ports
        -> memory/postgres/dict/fraud/spi/ratelimit adapters
     -> versioned payment events
```

The handler decodes JSON, authenticates an API key, adds request/correlation context, calls a use case, maps domain errors to HTTP, and renders a response. It must not decide fraud, change payment status, perform idempotency lookup, write SQL, or call SPI directly.

## Ports Declared In Use Cases

`internal/switcher` declares the ports it needs:

- `Store`: transfer state, idempotency, claims, audit, outbox, and settlement persistence.
- `ParticipantResolver`: DICT-like participant lookup.
- `FraudScorer`: fraud score and decision.
- `SPIClient`: payment-network submission.
- `RateLimiter`: pressure control by key.

Adapters implement these ports implicitly. The use case does not import `internal/postgres`, `internal/store`, `internal/dict`, `internal/fraud`, `internal/spi`, or `internal/ratelimit`.

## Domain Ownership

`internal/rail` owns:

- transfer request validation and fingerprinting
- settlement callback fingerprinting
- SPI claim, SPI approval, review, and settlement state transitions
- sentinel errors used by adapters and primary adapters
- audit record shape as payment-switch evidence, not as a SQL row model

The database model is not the domain. PostgreSQL scans rows into domain state at the adapter boundary, then calls domain transition methods before writing updates.

## Event Boundary

Events are versioned contracts under `docs/events/*.v1.json` and executable envelopes in `internal/events`. Use cases emit events next to transfer state changes; adapters persist or publish them. A broker adapter can be added without moving event decision logic into handlers or SQL code.

## Honest Gaps

- Broker-backed publishing is still represented by a publisher port and local relay tests.
- Redis-backed distributed rate limiting is not implemented; local algorithms and cache codecs are ready for a future adapter.
- Real DICT/SPI providers are simulated. Provider adapters must satisfy existing ports and add provider-specific contract tests.
