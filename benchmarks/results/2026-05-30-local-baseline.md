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
BenchmarkCreateTransfer-10        40576    30457 ns/op    27849 B/op    230 allocs/op
PASS
```

Latency profile output:

```text
local profile iterations=250 p50=18.541us p95=35.666us p99=260.542us throughput=33744 rps error_rate=0.00%
```

| Metric | Result |
| --- | ---: |
| p50 latency | 18.541 microseconds |
| p95 latency | 35.666 microseconds |
| p99 latency | 260.542 microseconds |
| mean in-process HTTP latency | 30.457 microseconds/op |
| throughput | 33,744 rps in latency profile; about 32,833 ops/sec in benchmark process |
| error rate | 0% for benchmarked happy path |
| memory | 27,849 B/op |
| allocations | 230 allocs/op |
| CPU/memory notes | no external IO; allocations mostly JSON and response maps |

k6 smoke, load, stress, and spike scripts are included but should be run against a long-lived local server or Compose environment before using them as release SLO evidence.
