# Scalability

## Hot path

`POST /v1/pix/transfers` is the hot path. It performs authentication, idempotency lookup, rate-limit checks, DICT lookup, fraud scoring, SPI message creation, transfer persistence, audit append, and outbox append.

## Read-heavy paths

- `GET /v1/pix/transfers/{id}` for operational lookup
- `GET /metrics` for Prometheus scraping
- future outbox/admin dashboards

## Write-heavy paths

- transfer creation
- settlement callbacks
- outbox relay updates
- audit record append

## Fastest-growing data

1. `payment_outbox`
2. `audit_records`
3. `pix_transfers`
4. processed callback/idempotency evidence

## Queue buildup risk

The outbox grows first when downstream broker publishing fails. Relay attempts, last error, and available-at timestamps make this visible and retryable.

## Hot partitions

`account_id` is the ordering partition. Large merchants or payout accounts can become hot partitions. The first mitigation is tenant/account rate limiting; later mitigations include account sub-partitions for event consumers that do not require strict payer-local order.

## Horizontal scaling

The HTTP API is horizontally scalable when idempotency, transfer state, and outbox use PostgreSQL, and rate limiting moves to Redis. In-memory mode is local/testing only.

## Sharding candidates

- `pix_transfers` by tenant or account after single-primary limits are reached
- `payment_outbox` by account partition or event type
- analytics projection by event date and tenant

## Async candidates

- event publication
- analytics projection
- fraud model enrichment
- non-blocking notification/webhook delivery

## Must not be eventual

- idempotent transfer creation
- fraud decision before SPI routing
- terminal settlement transition
- outbox append relative to transfer decision
