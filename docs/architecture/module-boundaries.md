# Module Boundaries

| Module | Responsibility | Must not own |
| --- | --- | --- |
| `internal/api` | HTTP routing, auth, error envelopes, request observability | domain decisions |
| `internal/switcher` | transfer orchestration and invariants | HTTP or database details |
| `internal/rail` | domain types and validation | provider IO |
| `internal/dict` | receiver key lookup adapter | fraud policy |
| `internal/fraud` | scoring and decision rules | settlement callback state |
| `internal/spi` | SPI message creation adapter | ledger postings |
| `internal/store` | local memory persistence | SQL concerns |
| `internal/postgres` | durable PostgreSQL persistence | business rules |
| `internal/messaging` | topology and relay behavior | transfer decisions |
| `internal/observability` | metrics and traces | domain state mutation |
