# Deployment View

## Local

- Go API process
- memory store
- fake DICT/SPI adapters
- Prometheus optional through Compose

## Durable single-region

- Go API process
- optional separate outbox relay process
- PostgreSQL for transfer, outbox, audit, and callback evidence
- Redis for distributed rate limiting when added
- broker for payment-rail event delivery when added
- Prometheus and Grafana for metrics and dashboarding

## Production guardrails

`PIXRAIL_ENV=production` requires configured API keys and PostgreSQL storage. Memory mode is rejected because payment decisions must survive process restarts.
