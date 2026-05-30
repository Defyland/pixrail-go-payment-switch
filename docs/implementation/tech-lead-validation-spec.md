# Tech Lead Validation Spec

This document is the spec-driven review for raising PixRail from a strong portfolio MVP to a repository that can credibly validate senior/tech-lead judgment.

## Executive assessment

PixRail already demonstrates several senior signals:

- clear bounded context: payment rail instead of ledger
- versioned HTTP API with OpenAPI
- idempotent transfer intake
- tenant-scoped API key authorization
- DICT, fraud, SPI, outbox, metrics, traces, and runbooks
- tests across domain, API, authorization, messaging topology, and failure scenarios
- atomic Conventional Commit history

The remaining gap is not feature count. The gap is production realism: persistence, dependency readiness, outbox delivery semantics, and explicit operational trade-offs. A strong evaluator will look for whether the author can separate a runnable demo from production constraints and close the highest-risk gaps first.

## Technical gaps that mattered

| Gap | Why it matters to senior evaluation | Required improvement |
| --- | --- | --- |
| In-memory store only | Good for tests, weak for payment-state durability | Add PostgreSQL migration and store adapter with transactional boundaries |
| Static readiness endpoint | Misleading readiness is an operational incident source | Readiness must depend on store health |
| Outbox only inserted, not relayable | Event-driven architecture is incomplete without retry, ack, and retry scheduling | Add relay contract with publish, mark-published, mark-failed, retry backoff |
| Production config allowed memory | A production boot path using volatile state undermines the architecture | Make production require configured API keys and PostgreSQL DSN |
| Technical assessment missing | Reviewers need to see what trade-offs are intentional versus unfinished | Document senior validation, residual risks, and roadmap by priority |

## Implementation acceptance criteria

1. Runtime can select `memory` or `postgres` storage through configuration.
2. Production mode fails fast unless API keys and PostgreSQL DSN are configured.
3. PostgreSQL schema documents and enforces:
   - unique `(tenant_id, idempotency_key)`
   - unique SPI message IDs
   - unique event IDs
   - transfer, audit, outbox, and callback persistence
   - pending outbox indexes
4. Readiness returns unavailable when storage health fails.
5. Outbox relay supports:
   - pending event selection
   - publisher acknowledgement
   - retry scheduling with attempt count and error evidence
   - correlation-preserving event publication
6. Tests prove:
   - memory store relay behavior
   - readiness health behavior
   - PostgreSQL migration contract
   - configuration safety for production
7. Documentation explains:
   - what improved technically
   - why those choices matter
   - what should still be done next for an excellent hiring signal

## Non-goals for this pass

- real DICT or SPI provider certification
- Kubernetes manifests
- multi-region active-active processing
- full Event Sourcing
- real broker integration in local tests

These are intentionally deferred because persistence, readiness, and outbox relay are the higher-priority production-readiness gaps for the current repository shape.
