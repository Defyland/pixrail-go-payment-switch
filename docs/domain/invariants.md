# Domain Invariants

1. A tenant cannot read or mutate another tenant's transfer.
2. A `(tenant_id, idempotency_key)` pair maps to one transfer.
3. A blocked transfer never creates a SPI message.
4. An approved transfer has a SPI message ID and end-to-end ID.
5. A settlement callback must match the transfer SPI message ID.
6. A terminal transfer cannot be settled or rejected again.
7. Transfer state, audit evidence, and outbox events are one logical transaction.
8. Outbox relay never changes event payload after creation.
9. Consumers must deduplicate by `event_id`.
10. PixRail events are not ledger postings.

Tests cover the highest-risk invariants in `internal/switcher/service_test.go`, `internal/store/memory_test.go`, and `internal/api/server_test.go`.
