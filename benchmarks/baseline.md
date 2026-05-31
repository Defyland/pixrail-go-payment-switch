# PixRail Benchmark Baseline

Baseline target for local MVP:

| Metric | Target |
| --- | ---: |
| create transfer p50 | < 5 ms |
| create transfer p95 | < 20 ms |
| create transfer p99 | < 50 ms |
| error rate under load | < 1% excluding intentional 429 |
| steady throughput | 100 rps on local development hardware |

Baseline local results are documented in [results/2026-05-30-local-baseline.md](results/2026-05-30-local-baseline.md).
The latest hardening smoke is documented in [results/2026-05-31-hardening-smoke.md](results/2026-05-31-hardening-smoke.md).
The k6 scripts run tagged warmup traffic before measured traffic and apply latency/error thresholds only to `phase:measured`, so cold-start noise is visible in the total summary without poisoning steady-state p95/p99 gates.
