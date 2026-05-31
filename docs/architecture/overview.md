# Architecture Overview

PixRail is a payment switch optimized for the transfer decision hot path. It keeps product behavior close to production concerns while avoiding external dependencies in the local MVP.

Architecturally, PixRail is a Go modular monolith with Hexagonal/Ports & Adapters boundaries. It is not MVC renamed: HTTP, CLI, and worker processes are adapters; `internal/switcher` owns use case orchestration; `internal/rail` owns domain invariants and payment state transitions; persistence/provider/cache/broker implementations are replaceable adapters.

## Runtime modules

| Module | Responsibility |
| --- | --- |
| HTTP API | versioned endpoints, API key auth, error envelope, request/correlation IDs |
| Switch service | orchestration, idempotency, rate limits, DICT, fraud, SPI, outbox |
| DICT resolver | deterministic receiver lookup and failure simulation |
| Fraud engine | rules-based score and decision log |
| SPI simulator | SPI message ID and end-to-end ID generation |
| Store | transfer state, idempotency index, outbox, audit log |
| SPI worker | long-running polling process for accepted transfers that need SPI submission |
| Observability | logs, metrics, traces, health, readiness |

Detailed architecture docs:

- [Ports and Adapters](ports-and-adapters.md)
- [Go Architecture](go-architecture.md)
- [Dependency Rule](dependency-rule.md)
- [Module Boundaries](module-boundaries.md)
- [Testing Strategy](testing-strategy.md)

## Boundaries

PixRail owns payment-rail evidence. It does not own balances, ledger postings, settlement accounting, chargebacks, or reconciliation. Those belong to a financial core such as SettleFlow.

## Deferred complexity

The service is a modular monolith before microservices because transfer state, audit, and outbox writes need a clear transaction boundary. PostgreSQL is implemented for durable state; Redis-backed rate limits and broker-backed publishing remain explicit next steps, but their absence does not weaken the domain boundary.
