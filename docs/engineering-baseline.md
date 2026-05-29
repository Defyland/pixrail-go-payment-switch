# PixRail Engineering Baseline

This repository follows the initiative-wide standards below.

## Mandatory outcomes

- product-grade `README.md` with product and engineering sections
- `openapi.yaml` once the HTTP surface exists
- `docs/adr/`, `docs/architecture/`, `docs/events/`, `docs/benchmarks/`, `docs/api/`, `docs/diagrams/`, and `docs/runbooks/`
- atomic Conventional Commit history
- GitHub Actions for lint, tests, security, build, coverage, and OpenAPI validation
- observability with structured logs, metrics, traces, request IDs, and readiness endpoints
- documented k6 performance baselines

## PixRail-specific emphasis

- hot-path Pix routing that remains distinct from the ledger and settlement core
- idempotent transfer creation, DICT lookup, SPI message handling, and event consumption
- leaky-bucket or token-bucket enforcement for tenant, account, and DICT traffic
- antifraud scoring with persisted decision logs, triggered rules, and replay-safe outcomes
- ordering guarantees by account or transfer identity with duplicate and out-of-order handling
- event streaming boundaries compatible with Redpanda, ClickHouse, and later analytical consumers

## Phase 0 boundary

This repository intentionally stops before scaffolding Go services, brokers, DICT adapters, or SPI simulators. The goal of this phase is only to lock scope and standards.
