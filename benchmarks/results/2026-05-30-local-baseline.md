# 2026-05-30 Local Baseline

Environment:

- machine: local developer workstation
- runtime: Go 1.25.10
- adapters: in-memory store, static DICT, rules fraud engine, SPI simulator

Native benchmark command:

```sh
go test -bench=. -benchmem ./internal/api
```

Latency budget command:

```sh
go test -run TestCreateTransferLatencyBudget -v ./internal/api
```

Measured output:

```text
goos: darwin
goarch: arm64
pkg: github.com/Defyland/pixrail-go-payment-switch/internal/api
cpu: Apple M1 Max
BenchmarkCreateTransfer-10        44866    29650 ns/op    27248 B/op    230 allocs/op
PASS
```

Latency profile output:

```text
local profile iterations=250 p50=19.416us p95=75.834us p99=325.583us throughput=28561 rps error_rate=0.00%
```

| Metric | Result |
| --- | ---: |
| p50 latency | 19.416 microseconds |
| p95 latency | 75.834 microseconds |
| p99 latency | 325.583 microseconds |
| mean in-process HTTP latency | 29.650 microseconds/op |
| throughput | 28,561 rps in latency profile; about 33,727 ops/sec in benchmark process |
| error rate | 0% for benchmarked happy path |
| memory | 27,248 B/op |
| allocations | 230 allocs/op |
| CPU/memory notes | no external IO; allocations mostly JSON and response maps |

k6 smoke, load, stress, and spike scripts are included but should be run against a long-lived local server or Compose environment before using them as release SLO evidence.
