# 2026-05-30 Local Baseline

Environment:

- machine: local developer workstation
- runtime: Go 1.25.10
- adapters: in-memory store, static DICT, rules fraud engine, SPI simulator

Native benchmark command:

```sh
go test -bench=. -benchmem ./internal/api
```

Measured output:

```text
goos: darwin
goarch: arm64
pkg: github.com/Defyland/pixrail-go-payment-switch/internal/api
cpu: Apple M1 Max
BenchmarkCreateTransfer-10        48748    24339 ns/op    28111 B/op    229 allocs/op
PASS
```

| Metric | Result |
| --- | ---: |
| mean in-process HTTP latency | 24.339 microseconds/op |
| throughput estimate | about 41,086 ops/sec in benchmark process |
| error rate | 0% for benchmarked happy path |
| memory | 28,111 B/op |
| allocations | 229 allocs/op |
| CPU/memory notes | no external IO; allocations mostly JSON and response maps |

k6 smoke, load, stress, and spike scripts are included but should be run against a long-lived local server or Compose environment before using them as release SLO evidence.
