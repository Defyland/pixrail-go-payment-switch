# ADR 0004: Use PostgreSQL for Durable Payment State

## Status

Accepted.

## Context

The first PixRail implementation used an in-memory store for fast local execution. That is useful for tests and demos, but it is not credible for payment state, idempotency, audit evidence, or outbox durability.

## Options considered

1. Keep in-memory storage for all modes.
2. Use broker messages as the only durable record.
3. Use PostgreSQL for transfer, audit, callback, and outbox state.

## Decision

Use PostgreSQL as the durable store. Keep memory mode for local development and tests only. Production configuration must select PostgreSQL.

## Consequences

Positive:

- durable idempotency evidence
- transactional transfer, audit, and outbox writes
- familiar indexing, backup, and migration story

Negative:

- requires migrations and connection-pool operations
- introduces database availability into readiness
- changes benchmark profile compared with memory mode
