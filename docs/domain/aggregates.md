# Aggregates

## Pix Transfer

Aggregate root: `Transfer`.

Consistency boundary:

- idempotency key belongs to `(tenant_id, idempotency_key)`
- request fingerprint belongs to the idempotency key and rejects mismatched replay
- fraud decision and SPI identifiers belong to the transfer
- SPI identifiers can only be recorded after the transfer is already accepted
- settlement callback can only mutate the matching transfer
- terminal transfer cannot transition again unless the callback hash is the same replay

## Outbox Record

Aggregate root: `OutboxRecord`.

Consistency boundary:

- event ID is globally unique
- event belongs to a transfer and tenant/account partition
- relay attempts mutate publish state but not event payload

## Audit Record

Audit records are append-only evidence. They are not edited to fix state; new evidence must be appended.
