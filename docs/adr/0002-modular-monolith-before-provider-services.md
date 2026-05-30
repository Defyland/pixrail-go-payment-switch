# ADR 0002: Modular Monolith Before Provider Services

## Status

Accepted.

## Context

PixRail has multiple natural provider boundaries: DICT, fraud, SPI, rate limiting, storage, and broker delivery. Splitting these into deployable services before the transaction model is stable would increase operational complexity without improving the MVP.

## Decision

Keep PixRail as a modular monolith with explicit ports and adapters. Provider implementations can move behind network boundaries later without changing the payment switch contract.

## Consequences

- tests can exercise full transfer behavior in process
- transfer, audit, and outbox consistency remains easy to reason about
- deployment is simpler while the product proves the hot path
- adapter boundaries remain visible for later extraction
