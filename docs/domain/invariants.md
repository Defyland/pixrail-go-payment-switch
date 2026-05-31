# Domain Invariants

1. A tenant cannot read or mutate another tenant's transfer.
2. A `(tenant_id, idempotency_key)` pair maps to one transfer.
3. Reusing an idempotency key with a different request fingerprint is a conflict.
4. A blocked transfer never creates a SPI message.
5. An accepted transfer has no SPI message ID yet and is safe to retry before submission.
6. An approved transfer has a SPI message ID and end-to-end ID.
7. A settlement callback must match the transfer SPI message ID.
8. A duplicate settlement callback replays only when the callback hash matches the processed callback.
9. A terminal transfer cannot be settled or rejected with a different callback payload.
10. Transfer state, audit evidence, request fingerprint, and outbox events are one logical transaction.
11. Outbox relay never changes event payload after creation.
12. Consumers must deduplicate by `event_id`.
13. PixRail events are not ledger postings.

Tests cover the highest-risk invariants in `internal/switcher/service_test.go`, `internal/store/memory_test.go`, and `internal/api/server_test.go`.
