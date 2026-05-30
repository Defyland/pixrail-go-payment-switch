# ADR 0003: Use Outbox Before Full Event Sourcing

## Status

Accepted.

## Context

PixRail emits payment-rail events, but it is not the financial ledger. Full Event Sourcing would add replay and projection complexity to a service whose primary job is low-latency transfer decisioning.

## Decision

Use transfer rows plus an outbox as the consistency boundary. Events are public integration contracts, not the only persisted source of state.

## Consequences

- downstream consumers receive reliable event evidence
- duplicate delivery remains expected and testable
- transfer reads stay simple
- Event Sourcing can be revisited if provider replay and forensic requirements grow
