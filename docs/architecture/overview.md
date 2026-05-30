# Architecture Overview

PixRail is a payment switch optimized for the transfer decision hot path. It keeps product behavior close to production concerns while avoiding external dependencies in the local MVP.

## Runtime modules

| Module | Responsibility |
| --- | --- |
| HTTP API | versioned endpoints, API key auth, error envelope, request/correlation IDs |
| Switch service | orchestration, idempotency, rate limits, DICT, fraud, SPI, outbox |
| DICT resolver | deterministic receiver lookup and failure simulation |
| Fraud engine | rules-based score and decision log |
| SPI simulator | SPI message ID and end-to-end ID generation |
| Store | transfer state, idempotency index, outbox, audit log |
| Observability | logs, metrics, traces, health, readiness |

## Boundaries

PixRail owns payment-rail evidence. It does not own balances, ledger postings, settlement accounting, chargebacks, or reconciliation. Those belong to a financial core such as SettleFlow.

## Deferred complexity

The service is a modular monolith before microservices because transfer state, audit, and outbox writes need a clear transaction boundary. PostgreSQL, Redis, and broker adapters are explicit next steps, but their absence does not weaken the domain boundary.
