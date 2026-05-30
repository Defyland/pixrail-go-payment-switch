# Incident Response

## High 5xx rate

1. Check `/healthz`, `/readyz`, and `/metrics`.
2. Filter logs by `correlation_id` and `request_id`.
3. Separate validation or rate-limit errors from dependency failures.
4. If DICT timeout simulation appears, inspect receiver keys containing `timeout`.
5. If settlement conflicts increase, inspect SPI message IDs and duplicate callback patterns.

## Recovery

- reduce incoming traffic at the tenant gateway
- increase bucket capacity only after confirming abuse is not active
- replay outbox records by account partition after downstream recovery
- preserve audit records; never rewrite fraud decision evidence
