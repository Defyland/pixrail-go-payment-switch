# PixRail Event Contracts

PixRail is the payment rail, not the financial ledger. PixRail decides, routes, rate-limits, and emits payment-rail events. SettleFlow remains responsible for ledger, balances, and financial settlement records.

## Envelope

Every payment-rail event must include:

- `event_id`
- `event_type`
- `schema_version`
- `occurred_at`
- `producer`
- `tenant_id`
- `account_id`
- `pix_transfer_id`
- `correlation_id`
- `payload`

## Ordering and idempotency

- Partition by `account_id` for payer-local ordering.
- Consumers deduplicate by `event_id`.
- `end_to_end_id`, SPI message IDs, and idempotency keys are unique business identifiers.
- Duplicate or out-of-order SPI callbacks must append evidence without duplicating state transitions.
- Relay workers claim outbox records with a token and lease before publishing; a publish acknowledgement or retry update is accepted only from the owning claim.

## Versioning and compatibility

- `schema_version` is a string and starts at `"1"`.
- Producers may add optional fields to `payload`.
- Producers must not remove required envelope fields or change field meaning without a new schema version.
- Consumers must ignore unknown optional payload fields.
- Consumers must tolerate duplicate delivery and replay.

## Producer responsibilities

- write transfer state, audit evidence, and outbox events in the same transaction boundary
- propagate `correlation_id` from the HTTP request into every event
- keep `tenant_id`, `account_id`, and `pix_transfer_id` populated for isolation and partitioning
- publish only after durable outbox insert
- use the outbox claim/lease contract when multiple relay workers are running

## Consumer responsibilities

- deduplicate by `event_id` before side effects
- process account partitions in order when ordering matters
- send poison messages to the dead-letter queue after retries
- preserve trace context and correlation IDs in downstream logs

## Versioned schemas

- [pix_transfer_requested.v1.json](pix_transfer_requested.v1.json)
- [dict_key_resolved.v1.json](dict_key_resolved.v1.json)
- [fraud_score_calculated.v1.json](fraud_score_calculated.v1.json)
- [pix_transfer_accepted.v1.json](pix_transfer_accepted.v1.json)
- [spi_submission_requested.v1.json](spi_submission_requested.v1.json)
- [pix_transfer_approved.v1.json](pix_transfer_approved.v1.json)
- [pix_transfer_blocked.v1.json](pix_transfer_blocked.v1.json)
- [pix_transfer_review_requested.v1.json](pix_transfer_review_requested.v1.json)
- [pix_transfer_review_approved.v1.json](pix_transfer_review_approved.v1.json)
- [pix_transfer_review_blocked.v1.json](pix_transfer_review_blocked.v1.json)
- [spi_message_created.v1.json](spi_message_created.v1.json)
- [pix_transfer_settled.v1.json](pix_transfer_settled.v1.json)
- [pix_transfer_rejected.v1.json](pix_transfer_rejected.v1.json)
