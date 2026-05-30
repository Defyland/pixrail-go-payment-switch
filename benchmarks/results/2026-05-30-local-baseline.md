# 2026-05-30 Local Baseline

Environment:

- machine: local developer workstation
- runtime: Go 1.25.10
- native benchmark adapters: in-memory store, static DICT, rules fraud engine, SPI simulator
- k6 adapters: Compose API, PostgreSQL 17, migration runner, Prometheus, tracing exporter disabled

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
BenchmarkCreateTransfer-10        47500    27647 ns/op    28347 B/op    230 allocs/op
PASS
```

Latency profile output:

```text
local profile iterations=250 p50=19.5us p95=25.125us p99=53us throughput=38751 rps error_rate=0.00%
```

| Metric | Result |
| --- | ---: |
| p50 latency | 19.5 microseconds |
| p95 latency | 25.125 microseconds |
| p99 latency | 53 microseconds |
| mean in-process HTTP latency | 27.647 microseconds/op |
| throughput | 38,751 rps in latency profile; about 36,170 ops/sec in benchmark process |
| error rate | 0% for benchmarked happy path |
| memory | 28,347 B/op |
| allocations | 230 allocs/op |
| CPU/memory notes | no external IO; allocations mostly JSON and response maps |

k6 command base:

```sh
BASE_URL=http://127.0.0.1:18080 PIXRAIL_API_KEY=dev-secret k6 run benchmarks/k6/<profile>.js
```

| k6 profile | Result | Throughput | Latency |
| --- | ---: | ---: | ---: |
| smoke | 5/5 checks, 0% failures | 118.62 req/s | p95 20.74 ms |
| load | 8,666/8,666 checks, 0% failures | 72.20 req/s | p95 13.95 ms, p99 24.96 ms |
| stress | 60,530/60,530 checks, 0% failures | 504.85 req/s | p95 17.97 ms, p99 26.55 ms |
| spike | 576,597/576,597 checks, 0% failures | 11,532.44 req/s | p95 11.94 ms, p99 17.47 ms |

k6 load, stress, and spike treat `429` as an expected rate-limit response. The goal of those profiles is to prove bounded backpressure and latency under overload, not unlimited accepted payment creation.
