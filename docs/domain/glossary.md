# Domain Glossary

| Term | Meaning |
| --- | --- |
| PixRail | Payment switch responsible for Pix transfer decisions and routing. |
| Payment rail | Hot-path system that validates, routes, and emits payment-network events. |
| Ledger | Financial source of truth for balances and accounting; explicitly outside PixRail. |
| Tenant | Fintech or platform customer calling PixRail. |
| Account | Tenant-local payer account used as partition key. |
| Idempotency key | Client-provided key that prevents duplicate transfer creation. |
| DICT | Pix directory lookup for receiver keys. Simulated locally. |
| SPI | Pix settlement/payment network. Simulated locally. |
| End-to-end ID | Payment-network identifier assigned to approved transfers. |
| Fraud decision | Score, rules, status, and reason produced before SPI routing. |
| Outbox | Durable event buffer written with transfer state. |
| Relay | Worker that publishes outbox events and records ack/retry state. |
