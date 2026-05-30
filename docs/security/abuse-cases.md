# Abuse Cases

## DICT enumeration

An attacker submits many receiver keys to discover valid Pix keys.

Controls:

- tenant/account rate limits
- DICT lookup bucket
- audit and metrics for dependency pressure

## Idempotency abuse

A caller reuses keys to hide duplicate attempts or changes payload under one key.

Controls:

- unique `(tenant_id, idempotency_key)`
- replay returns the original transfer
- production persistence must store payload hash if mutable replay detection is required

## BOLA transfer lookup

Tenant A requests Tenant B's transfer ID.

Controls:

- all transfer lookups include authenticated tenant
- cross-tenant reads return `404`
- tests cover tenant isolation

## Callback spoofing

An attacker sends a fake SPI callback.

Controls:

- current local MVP requires tenant API key and SPI message ID match
- production must add signed callback verification

## Outbox replay abuse

Consumer receives duplicate events and applies side effects twice.

Controls:

- event consumers deduplicate by `event_id`
- outbox payload is immutable
- relay marks acknowledgement separately from event creation
