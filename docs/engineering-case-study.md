# Engineering Case Study

## 1. Product Context

PixRail is a Pix-like payment switch for fintech teams that need to validate transfer intake, receiver lookup, fraud decisioning, SPI routing, settlement callbacks, and event publication before provider certification. The product value is a credible hot-path payment rail that does not pretend to own balances or ledger accounting.

## 2. Domain Model

The main aggregate is `Transfer`. It carries tenant, account, idempotency key, receiver key, DICT result, fraud score, SPI identifiers, status, and settlement evidence. Outbox and audit records are separate evidence aggregates tied to the transfer.

## 3. Architecture

PixRail is a modular monolith with explicit ports for DICT, fraud, SPI, storage, rate limiting, observability, and outbox relay. This keeps the transfer transaction boundary understandable while leaving provider adapters replaceable.

## 4. Key Trade-offs

The project chooses local runnable adapters first and durable production design second. The accepted cost is that local memory mode is not production state. The benefit is a fast portfolio runtime with tests and honest docs about what must move to PostgreSQL, Redis, and a broker.

## 5. Data Model

Production data is modeled around `pix_transfers`, `payment_outbox`, `audit_records`, and processed callback/idempotency evidence. Unique constraints protect idempotency keys, event IDs, and SPI identifiers.

## 6. Consistency Model

Transfer state, audit evidence, and outbox events are one logical transaction. Settlement callbacks are guarded by tenant, transfer ID, SPI message ID, and terminal state checks. Event consumers must deduplicate by `event_id`.

## 7. Failure Scenarios

PixRail handles invalid JSON, validation failures, unauthorized requests, rate-limit exhaustion, DICT timeout simulation, fraud blocks, SPI callback conflicts, terminal callback replay, and outbox publish retries.

## 8. Performance Strategy

The hot path uses in-process rules and deterministic adapters for local benchmark evidence. The benchmark package reports p50, p95, p99, throughput, error rate, memory, and allocation data. Future persistent adapters should be benchmarked separately because IO changes the latency profile.

## 9. Scalability Strategy

The first scaling boundary is stateless HTTP plus shared PostgreSQL and Redis. The first backlog risk is outbox growth when downstream consumers fail. Account-level partitioning preserves payer-local order but can create hot partitions for large tenants.

## 10. Security Model

Security centers on API key tenant scoping, input validation, tenant isolation, rate limits, idempotency, audit evidence, and secret configuration. Signed provider callbacks are explicitly deferred.

## 11. Observability

PixRail emits structured logs, request IDs, correlation IDs, Prometheus metrics, OpenTelemetry spans, dashboard JSON, alerts, health, readiness, and runbooks. Readiness reflects storage health.

## 12. Operational Cost

The MVP keeps runtime cost low with one Go service. The production path adds PostgreSQL, Redis, broker, Prometheus, Grafana, backup, retention, migration, and replay operations. Those costs are documented instead of hidden.

## 13. Maintainability

Packages map to domain boundaries: `rail`, `switcher`, `dict`, `fraud`, `spi`, `store`, `outbox`, `api`, and `observability`. Tests use business language around idempotency, tenant isolation, fraud decisions, settlement, and relay behavior.

## 14. Product Decisions

PixRail optimizes payment-rail confidence, not banking completeness. It says no to ledger ownership, real provider certification, and analytics warehouse implementation in the MVP.

## 15. What I Would Do Next

1. Add PostgreSQL integration tests with Compose or Testcontainers.
2. Add broker-backed outbox publisher.
3. Add Redis-backed distributed rate limiting.
4. Add signed SPI callback verification.
5. Run k6 load/stress/spike against the durable adapter.
6. Add provider adapter contracts for real DICT and SPI certification.
