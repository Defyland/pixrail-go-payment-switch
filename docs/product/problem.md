# Product Problem

PixRail serves fintech product teams that need a payment-switch layer for Pix-like transfers before they connect to certified payment-network providers.

The core problem is not "send HTTP requests." The product problem is deciding whether a Pix transfer should be accepted, blocked, reviewed, routed to SPI, and published to downstream financial systems while preserving idempotency, tenant isolation, traceability, and operational evidence.

## Pain points

- Pix transfer intake must be low latency but still auditable.
- DICT lookups can become an abuse vector and must be rate-limited.
- Fraud decisions must explain why a transfer was approved or blocked.
- SPI callbacks can be duplicate or out of order.
- Downstream ledgers, risk systems, and analytics consumers need stable events.
- A payment rail must not accidentally become the financial ledger.

## Business value

PixRail lets a fintech team validate payment rail behavior, fraud controls, event contracts, observability, and failure handling before spending integration effort on real DICT, SPI, and banking providers.
