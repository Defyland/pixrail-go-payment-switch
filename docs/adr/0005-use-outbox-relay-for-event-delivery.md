# ADR 0005: Use Outbox Relay for Event Delivery

## Status

Accepted.

## Context

PixRail publishes events consumed by ledgers, risk systems, and analytics. Publishing synchronously during transfer creation would couple the payment hot path to downstream availability.

## Options considered

1. Synchronous downstream publish inside the transfer request.
2. Fire-and-forget goroutine after transfer creation.
3. Transactional outbox plus relay with retry evidence.

## Decision

Use a transactional outbox and a relay. The relay publishes pending events, marks acknowledged events as published, and records retry attempts and last errors when publishing fails.

## Consequences

Positive:

- transfer decisions do not depend on broker availability
- event delivery is retryable and auditable
- consumers still deduplicate by `event_id`

Negative:

- outbox backlog must be monitored
- relay operation adds another runtime responsibility
- poison messages require DLQ and replay procedures when a real broker is attached
