# Bounded Contexts

## Payment Rail

Owns transfer intake, idempotency, DICT resolution, fraud decisioning, SPI message creation, settlement callback state, audit evidence, and payment-rail events.

Code evidence:

- `internal/switcher`
- `internal/rail`
- `internal/dict`
- `internal/fraud`
- `internal/spi`

## Event Delivery

Owns pending outbox selection, publish acknowledgement, retry scheduling, and dead-letter expectations.

Code evidence:

- `internal/events`
- `internal/outbox`
- `internal/store`

## Financial Core

Owns balances, ledger entries, settlement accounting, reconciliation, refunds, and financial reporting. It consumes PixRail events but does not delegate accounting authority to PixRail.

This context is intentionally external to the repository.
