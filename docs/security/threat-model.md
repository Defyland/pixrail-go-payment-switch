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
- internal SPI worker with a worker-scoped API key
- risk analyst service with a risk-scoped API key
- payment-network/provider adapter with a provider-scoped API key
- compromised tenant backend replaying or flooding requests
- external payment-network dependency returning slow or inconsistent responses
- downstream consumer replaying events after recovery
- operator inspecting outbox and audit evidence during incidents

## Primary threats

| Threat | Control |
| --- | --- |
| DICT abuse | tenant, account, and key-lookup rate limits |
| SPI duplicate callback | unique SPI message IDs plus processed callback hash dedupe |
| Tenant key triggering operational side effects | role-scoped API keys for tenant, worker, risk, and provider endpoints |
| Duplicate SPI worker submission | persisted SPI claim token, claim TTL, bounded SPI submit timeout, and claim-token checked persistence |
| Duplicate outbox publish by concurrent relays | outbox claim token and lease checked before publish/failure updates |
| Fraud bypass | decision log with score, rules, DICT result, and correlation ID |
| Hot path overload | bounded workers, backpressure, and rate limits |
| Out-of-order status | account partitioning and state transition guards |
| Ledger confusion | SettleFlow remains the financial source of truth |

## Tested controls

- API key authentication rejects unauthenticated requests.
- API keys without the required endpoint role return `403`.
- Tenant isolation returns `404` for cross-tenant transfer reads.
- Idempotency returns the original transfer without duplicating outbox records.
- Idempotency rejects the same key with a different request fingerprint.
- Rate-limit exhaustion returns `429`.
- High-risk DICT signals block before SPI message creation.
- Transfer creation persists accepted state before SPI submission.
- SPI submission claims the accepted transfer before calling the SPI client and releases the claim with retry evidence on SPI failure.
- The outbox relay claims records before publishing and only marks records when the claim token matches.
- Terminal settlement callback replay requires the same callback hash.

## Secret management

Production must provide `PIXRAIL_API_KEYS=tenant_id:secret[:role|role]`. The local `dev-secret`, `worker-secret`, `risk-secret`, and `provider-secret` fallbacks are disabled when `PIXRAIL_ENV=production`.

## Residual risks

- Real SPI, DICT, and anti-fraud providers are simulated in the MVP.
- Real provider callbacks still need request signing before external integration.
- Service mesh is deferred; app-level idempotency and tracing are the primary controls.
- Full Event Sourcing is deferred because payment rail state is not the financial ledger.
