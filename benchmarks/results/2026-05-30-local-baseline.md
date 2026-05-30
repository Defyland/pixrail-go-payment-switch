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
BenchmarkCreateTransfer-10        47972    29396 ns/op    28263 B/op    230 allocs/op
PASS
```

Latency profile output:

```text
local profile iterations=250 p50=18.708us p95=26.959us p99=71.75us throughput=38195 rps error_rate=0.00%
```

| Metric | Result |
| --- | ---: |
| p50 latency | 18.708 microseconds |
| p95 latency | 26.959 microseconds |
| p99 latency | 71.75 microseconds |
| mean in-process HTTP latency | 29.396 microseconds/op |
| throughput | 38,195 rps in latency profile; about 34,018 ops/sec in benchmark process |
| error rate | 0% for benchmarked happy path |
| memory | 28,263 B/op |
| allocations | 230 allocs/op |
| CPU/memory notes | no external IO; allocations mostly JSON and response maps |

k6 smoke, load, stress, and spike scripts are included but should be run against a long-lived local server or Compose environment before using them as release SLO evidence.
