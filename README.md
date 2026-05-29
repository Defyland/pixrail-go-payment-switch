# PixRail

Real-time Pix payment rail and switch built in Go to showcase low-latency transaction routing, DICT resolution, SPI-style messaging, antifraud checks, rate limiting, and event-driven payment decisioning.

## Status

Phase 0 bootstrap only. This repository currently establishes naming, scope, documentation structure, and engineering expectations. It does not yet contain Go services, DICT clients, SPI simulators, broker consumers, or analytics projections.

## Product intent

PixRail is planned as the hot-path Pix rail that sits between a fintech application and the external payment network. It receives Pix intents, resolves destination keys through a DICT-like directory, applies rate limits, runs fraud checks, generates SPI or ISO20022-like messages, decides approve, review, or block outcomes, and publishes operational events for downstream financial systems.

## Planned stack

- Go
- PostgreSQL
- Redis
- Redpanda or Kafka
- ClickHouse
- OpenTelemetry
- Prometheus and Grafana
- Docker Compose
- k6
- Apache Flink as a later-phase analytical extension

## Engineering focus

This project is meant to demonstrate:

- low-latency Pix routing separated from the financial source of truth
- DICT-like key resolution with resilient timeout and duplicate-handling semantics
- rate limiting and backpressure controls for hot payment paths
- auditable antifraud decisions with rule triggers and idempotent event handling
- SPI-style message lifecycle simulation with ordering and replay safety
- event streaming foundations for analytics and downstream settlement systems

## Bootstrap contents

- repository initialized and synchronized with GitHub
- mandatory documentation folders created, including `docs/events/`
- baseline engineering spec captured in `docs/engineering-baseline.md`

## Next phase

The first implementation slice should prioritize Pix transfer intake, DICT fake resolution, idempotent request handling, leaky-bucket limits, fraud decision logs, SPI message simulation, and event publication boundaries.
