# 2026-05-31 Hardening Smoke

Environment:

- machine: local developer workstation
- runtime: Go 1.25.10
- API mode: in-memory store, static DICT, rules fraud engine, SPI simulator
- k6 target: local API on `127.0.0.1:18081`
- tracing exporter: disabled for benchmark clarity

Commands:

```sh
GOCACHE=$PWD/.gocache go test -bench=. -benchmem ./internal/api
GOCACHE=$PWD/.gocache go test -run TestCreateTransferLatencyBudget -v ./internal/api
BASE_URL=http://127.0.0.1:18081 PIXRAIL_API_KEY=dev-secret k6 run benchmarks/k6/smoke.js
```

API benchmark:

```text
BenchmarkCreateTransfer-10    43267    27636 ns/op    27998 B/op    221 allocs/op
```

Latency profile:

```text
local profile iterations=250 p50=18.542us p95=56.833us p99=203.292us throughput=30663 rps error_rate=0.00%
```

k6 smoke:

| Metric | Result |
| --- | ---: |
| checks | 5/5 |
| measured error rate | 0.00% |
| measured p95 | 679.8 microseconds |
| measured throughput | 380.80 iterations/sec |

The k6 scripts tag warmup traffic as `phase:warmup` and measured traffic as `phase:measured`. Thresholds use the measured tag so cold-start behavior stays visible in the total summary without corrupting steady-state gates.
