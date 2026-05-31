# Deployment View

## Local

- Go API process
- memory store
- fake DICT/SPI adapters
- Prometheus optional through Compose

## Durable single-region

- Go API process
- Go SPI worker process running `cmd/pixrail-worker`
- optional separate outbox relay process
- PostgreSQL for transfer, outbox, audit, and callback evidence
- Redis for distributed rate limiting when added
- broker for payment-rail event delivery when added
- Prometheus and Grafana for metrics and dashboarding

## Production guardrails

`PIXRAIL_ENV=production` requires configured API keys and PostgreSQL storage. Memory mode is rejected because payment decisions must survive process restarts. Trace exporting defaults to `none` in production unless `PIXRAIL_TRACING_EXPORTER=stdout` is explicitly selected for local diagnostics. The worker polls accepted transfers with `PIXRAIL_WORKER_INTERVAL` and `PIXRAIL_WORKER_BATCH_SIZE`, and SPI claims prevent duplicate submissions across multiple worker replicas.
