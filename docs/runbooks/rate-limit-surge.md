# Rate Limit Surge

Use this runbook when `pixrail_http_requests_total{status="429"}` rises.

## Triage

- identify the tenant and account from request logs
- confirm whether the same receiver key is causing DICT bucket pressure
- compare 429 rate against successful transfer rate
- inspect whether traffic is retrying without backoff

## Mitigation

- ask the caller to apply exponential backoff with jitter
- temporarily raise tenant bucket size only for trusted internal tests
- block abusive API keys at the gateway if traffic is hostile
- keep idempotency keys stable during client retries
