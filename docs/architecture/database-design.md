# Database Design

The current implementation supports an in-memory store for local execution and tests plus a PostgreSQL adapter for durable payment-rail state. PostgreSQL migrations are versioned under `db/migrations/` and applied by `go run ./cmd/pixrail-migrate`.

## Migration Discipline

- `schema_migrations` records the migration version, file name, SHA-256 checksum, and application timestamp.
- Migrations are applied in sorted filename order inside one transaction per migration.
- Previously applied checksums are validated on every run; applied migration files are immutable.
- New schema changes must use a new `NNNN_description.sql` file. Do not edit applied migrations.

Current migrations:

| Version | File | Purpose |
| --- | --- | --- |
| `0001` | `0001_pixrail_core.sql` | Core transfers, outbox, audit, and processed callback tables. |
| `0002` | `0002_consistency_hardening.sql` | Forward-compatible consistency columns for databases created before request hashes and callback hashes. |
| `0003` | `0003_worker_leases.sql` | SPI submission and outbox claim leases for multi-worker safety. |

## Tables

Core tables:

- `pix_transfers`: tenant-scoped transfer state, idempotency key, request hash, fraud decision, SPI identifiers, SPI claim lease, settlement status, and timestamps.
- `payment_outbox`: durable event envelope, publish status, retry evidence, and relay claim lease.
- `audit_records`: immutable operational evidence for transfer decisions, review actions, SPI submission recording, and settlement callbacks.
- `processed_spi_callbacks`: callback hash dedupe keyed by tenant, SPI message ID, and callback hash.
- `schema_migrations`: migration runner metadata and checksum guard.

Key constraints and indexes:

- `unique (tenant_id, idempotency_key)` prevents duplicate transfer creation per tenant.
- `end_to_end_id` and `spi_message_id` are unique when present.
- `processed_spi_callbacks` prevents duplicate callback hashes and rejects conflicting terminal callbacks.
- `pix_transfers_spi_claim_idx` supports scanning unsubmitted accepted transfers whose claim lease expired.
- `payment_outbox_claim_idx` supports concurrent relay workers claiming available records.

## Transaction Boundaries

- Create transfer: insert transfer, request hash, audit record, and outbox events in one transaction.
- SPI claim: lock the accepted transfer, store `spi_claim_token`, `spi_claimed_until`, increment submission attempts, and clear previous SPI error before calling the SPI client.
- SPI submission record: lock the transfer again and persist SPI identifiers, approval events, and audit only when the worker claim token matches.
- SPI submission release: on SPI client error, clear the token, retain last error, and move `spi_claimed_until` to the retry time.
- Manual review: lock and transition a `review` transfer to `accepted` or `blocked`, with audit and outbox evidence.
- Settlement callback: lock transfer by tenant and ID, validate SPI message ID, guard terminal states, update transfer, insert audit, insert settlement event, and persist callback hash.
- Outbox relay: claim available records with `FOR UPDATE SKIP LOCKED`, publish through the publisher port, then mark `published` or schedule retry only when the claim token still matches.
- Migration runner: apply each migration transactionally against `PIXRAIL_DATABASE_URL`.

## Isolation and Rollback

Default `READ COMMITTED` is acceptable because state transitions use unique constraints, row-level locks, and claim tokens. Failed DICT or fraud calls happen before a transfer transaction, so no partial transfer row is written. Create never calls SPI. Failed outbox insert rolls back the transfer decision. Settlement callback replay is guarded by `processed_spi_callbacks.callback_hash`, so a different terminal callback payload is a conflict.

SPI submission is at-least-once at the service boundary and claim-protected locally. A real SPI adapter must pass a provider-supported idempotency key derived from the transfer ID; without an external provider contract, the local simulator provides deterministic message IDs and the service bounds submit time below the claim TTL.

## Runtime Guardrails

- `PIXRAIL_STORE_DRIVER=memory` is local/test only.
- `PIXRAIL_STORE_DRIVER=postgres` requires `PIXRAIL_DATABASE_URL`.
- `PIXRAIL_ENV=production` rejects memory storage and requires configured API keys.
- `PIXRAIL_API_KEYS` should separate tenant, worker, risk, and provider roles.
