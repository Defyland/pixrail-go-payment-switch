# Testing Strategy

PixRail tests match the architecture boundaries. The goal is not only coverage; it is proof that domain, use cases, adapters, and contracts can be validated independently.

## Test Layers

| Layer | Packages | What is proven |
| --- | --- | --- |
| Domain tests | `internal/rail` | Request validation, fingerprints, terminal statuses, SPI/review/settlement state machine rules. |
| Use case tests | `internal/switcher` | Idempotency, post-persist SPI submission, claim-before-side-effect, review resolution, settlement replay, fake port orchestration. |
| Adapter tests | `internal/store`, `internal/postgres`, `internal/dict`, `internal/spi`, `internal/ratelimit`, `internal/codec` | Persistence behavior, local simulations, algorithm behavior, binary contracts. |
| Primary adapter tests | `internal/api` | Auth, HTTP DTO mapping, request/correlation IDs, status codes, signed callbacks, metrics. |
| Event contract tests | `internal/spec`, `internal/events` | Versioned schemas, required envelope fields, strict payloads. |
| Architecture tests | `internal/spec/architecture_spec_test.go` | Dependency rule and required architecture documentation. |
| Performance tests | `benchmarks/k6`, benchmark tests | Hot-path latency, allocation profile, rate-limit and serialization costs. |

## What Must Stay True

- Handler tests should not be the only proof of payment behavior.
- Domain tests must run without HTTP, database, cache, or broker.
- Use case tests must run with fake adapters or local adapter fakes.
- Adapter integration tests may use PostgreSQL, but those tests must not define payment rules.
- Event schemas must be versioned and strict.
- Architecture tests must fail if a use case starts importing infrastructure.

## Current Evidence

- `internal/rail/model_test.go` tests domain state transitions without a store.
- `internal/switcher/service_test.go` tests use case orchestration with fake ports and local adapters.
- `internal/postgres/store_integration_test.go` tests the durable adapter when a DSN is supplied.
- `internal/spec/repository_spec_test.go` and `internal/spec/architecture_spec_test.go` make repository evidence executable.

## Gaps

- Real provider adapters will need provider contract tests when DICT/SPI endpoints exist.
- Broker-backed publishing will need publisher contract tests and DLQ replay tests.
- Redis-backed distributed rate limiting will need cross-process consistency tests.
