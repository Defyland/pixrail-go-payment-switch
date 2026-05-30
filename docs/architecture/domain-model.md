# Domain Model

## Transfer

Tenant-scoped payment intent with idempotency key, account ID, amount, receiver key, DICT evidence, fraud decision, SPI message identifiers, and settlement state.

Statuses:

- `approved`: fraud accepted and SPI message created
- `review`: fraud score requires manual review; no SPI message
- `blocked`: fraud policy blocked before SPI
- `settled`: SPI callback accepted
- `rejected`: SPI callback rejected

## DictEntry

Resolved receiver identity: receiver ID, bank ISPB, account hash, risk signal, and timestamp. The local resolver is deterministic for repeatable tests.

## FraudDecision

Score, triggered rule IDs, decision status, and reason. Current rules include high amount, review amount, high-risk DICT signal, and self-transfer hash match.

## Event

CloudEvents-like payment-rail envelope with transfer partition fields and a JSON payload. Events are inserted into the outbox in the same logical transaction as transfer state changes.
