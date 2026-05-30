# Operational Cost

## Infrastructure components

| Component | Current role | Cost accepted |
| --- | --- | --- |
| Go API | payment switch runtime | simple deployable unit |
| PostgreSQL | durable transfer, audit, and outbox state | backups, migrations, connection pooling |
| Redis | planned distributed rate limit backend | memory sizing, eviction monitoring |
| Broker | planned outbox destination | DLQ operations, replay procedures |
| Prometheus/Grafana | metrics, alerts, dashboards | dashboard and alert maintenance |

## Debugging cost

PixRail deliberately carries request ID, correlation ID, transfer ID, event ID, and SPI message ID so incidents can be traced across API logs, outbox records, and downstream consumers. The cost is stricter propagation discipline in every adapter.

## Deployment cost

A modular monolith has lower deployment cost than early microservices. The cost is that module boundaries must stay clear in code and tests; otherwise the monolith becomes harder to split later.

## Backup and retention

- transfer records: retained for operational and compliance investigation
- audit records: append-only retention, longer than ordinary logs
- outbox records: retained until published and then archived or compacted after replay window
- metrics: retained by Prometheus according to operational needs

## Monitoring burden

Required alerts:

- 5xx rate
- 429 surge
- DICT dependency failures
- outbox pending age
- relay failure rate
- settlement conflict rate

## Vendor lock-in risk

PostgreSQL and Redis are intentionally generic. A broker adapter should keep event publishing decoupled from Kafka, Redpanda, RabbitMQ, or cloud-native queues.

## Simpler alternatives rejected

- In-memory state in production: rejected because payment decisions require durability.
- Synchronous downstream calls instead of outbox: rejected because ledger/analytics outages would block the payment rail.
- Microservices in the MVP: rejected because they raise deployment and tracing cost before provider boundaries are stable.
