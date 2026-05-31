# GOMAXPROCS, Runtime Metrics, and Container Readiness

PixRail logs runtime CPU configuration at startup and exposes runtime metrics in `/metrics`.

## Implemented Controls

- API and worker startup logs include `gomaxprocs`, `num_cpu`, `store_driver`, `component`, and environment.
- `/metrics` includes:
  - `pixrail_runtime_gomaxprocs`
  - `pixrail_runtime_num_cpu`
  - `pixrail_runtime_goroutines`
  - `pixrail_runtime_heap_alloc_bytes`
- Optional pprof is controlled by `PIXRAIL_PPROF_ADDR`.
- PostgreSQL pool settings are explicit through:
  - `PIXRAIL_POSTGRES_MIN_CONNS`
  - `PIXRAIL_POSTGRES_MAX_CONNS`
  - `PIXRAIL_POSTGRES_MAX_CONN_LIFETIME`

## Kubernetes Guidance

Go 1.25 reads cgroup CPU limits better than older runtimes, but PixRail still logs `GOMAXPROCS` and `NumCPU` because p99 latency regressions often start with hidden CPU throttling.

Recommended production checks:

- CPU request should cover steady-state p95 load, not only average CPU.
- CPU limit should be high enough to avoid CFS throttling during Pix transfer bursts.
- Alert when p99 latency rises while `container_cpu_cfs_throttled_seconds_total` rises.
- Compare `pixrail_runtime_gomaxprocs` with expected pod CPU limit during deploys.
- Keep API and worker pprof endpoints private; bind only on localhost or a protected diagnostics network.

## Local Defaults

The local challenge runtime does not set `GOMAXPROCS`; it relies on the Go runtime and logs the effective value. This is intentional because artificial CPU tuning would make local benchmark evidence less portable.

## Production Gaps

- This repo does not ship Kubernetes manifests. That is a deployment packaging concern, not required for the local backend challenge.
- It does not include `automaxprocs`; adding it would be reasonable in a Kubernetes deployment, but the current runtime is already observable without another dependency.
