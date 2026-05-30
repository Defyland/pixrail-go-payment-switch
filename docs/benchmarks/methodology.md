# Benchmark Methodology

PixRail keeps two benchmark layers:

1. Native Go benchmark for the in-process HTTP hot path.
2. k6 smoke, load, stress, and spike scripts against a running server.

## Profiles

| Profile | Purpose | Script |
| --- | --- | --- |
| smoke | confirm one-user workflow and error budget shape | `benchmarks/k6/smoke.js` |
| load | steady traffic within expected capacity | `benchmarks/k6/load.js` |
| stress | push beyond expected capacity and observe rate limits | `benchmarks/k6/stress.js` |
| spike | sudden traffic burst and recovery | `benchmarks/k6/spike.js` |

## Metrics captured

- p50 latency
- p95 latency
- p99 latency
- throughput
- error rate
- rate-limit rate
- CPU and memory notes from local process or container stats

## Commands

```sh
go test -bench=. -benchmem ./internal/api
k6 run benchmarks/k6/smoke.js
k6 run benchmarks/k6/load.js
k6 run benchmarks/k6/stress.js
k6 run benchmarks/k6/spike.js
```
