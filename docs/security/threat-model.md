# Threat Model

## Assets

- Pix transfer intents and decisions
- DICT lookup responses and receiver-risk metadata
- rate-limit buckets and abuse evidence
- SPI-like messages and status callbacks
- fraud scores and decision logs
- outbox, inbox, and processed-event records

## Trust boundaries

- tenant APIs submit Pix transfer requests
- DICT and SPI simulators represent external payment-network dependencies
- fraud scoring receives transaction context and returns a decision
- downstream financial systems consume approved or blocked events

## Actors

- trusted tenant backend with an API key
- compromised tenant backend replaying or flooding requests
- external payment-network dependency returning slow or inconsistent responses
- downstream consumer replaying events after recovery
- operator inspecting outbox and audit evidence during incidents

## Primary threats

| Threat | Control |
| --- | --- |
| DICT abuse | tenant, account, and key-lookup rate limits |
| SPI duplicate callback | unique SPI message IDs and processed-event dedupe |
| Fraud bypass | decision log with score, rules, DICT result, and correlation ID |
| Hot path overload | bounded workers, backpressure, and rate limits |
| Out-of-order status | account partitioning and state transition guards |
| Ledger confusion | SettleFlow remains the financial source of truth |

## Tested controls

- API key authentication rejects unauthenticated requests.
- Tenant isolation returns `404` for cross-tenant transfer reads.
- Idempotency returns the original transfer without duplicating outbox records.
- Rate-limit exhaustion returns `429`.
- High-risk DICT signals block before SPI message creation.
- Terminal settlement callback replay does not duplicate transitions.

## Secret management

Production must provide `PIXRAIL_API_KEYS=tenant_id:secret`. The local `dev-secret` fallback is disabled when `PIXRAIL_ENV=production`.

## Residual risks

- Real SPI, DICT, and anti-fraud providers are simulated in the MVP.
- Service mesh is deferred; app-level idempotency and tracing are the primary controls.
- Full Event Sourcing is deferred because payment rail state is not the financial ledger.
