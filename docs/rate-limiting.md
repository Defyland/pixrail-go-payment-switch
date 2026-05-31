# Rate Limiting

PixRail uses rate limiting to protect the payment switch hot path, not to replace provider quotas or fraud controls. The code lives in `internal/ratelimit` and includes four local algorithms with deterministic tests and benchmarks.

## Algorithms

| Algorithm | Strength | Weakness | PixRail fit |
| --- | --- | --- | --- |
| Token bucket | Allows controlled bursts and steady refill | Burst can be too generous for strict provider quotas | Default for tenant/account transfer intake |
| Fixed window | Very cheap and easy to reason about | Boundary bursts at window rollover | Coarse admin/diagnostic endpoints |
| Sliding window log | Exact rolling-window enforcement | More memory per key and higher CPU | DICT receiver-key pressure and suspicious retry patterns |
| Leaky bucket | Smooths output rate | Less expressive for burst entitlement | SPI worker/provider submit pacing |

## Current Runtime Use

The API currently uses token buckets for:

- `POST /v1/pix/transfers` by `tenant_id:account_id`
- DICT lookup pressure by `tenant_id:receiver_key`

The other algorithms are implemented and tested for the next local hardening slices. They are intentionally not forced into all flows because a payment switch should choose the limiter by risk profile.

## Recommendation By Flow

| Flow | Recommended algorithm | Reason |
| --- | --- | --- |
| Tenant transfer creation | Token bucket | Allows product bursts while enforcing average tenant/account pressure. |
| DICT lookup by receiver key | Sliding window | Prevents repeated hot-key abuse across exact rolling intervals. |
| SPI worker submissions | Leaky bucket | Smooths provider-facing submit rate and avoids worker bursts after downtime. |
| Provider callbacks | Sliding window plus signature validation | Detects callback replay/flood attempts while HMAC validates authenticity. |
| `/metrics`, `/healthz`, `/readyz` | Network policy first, fixed window only if public | These are operational endpoints; do not put public traffic in front of them. |

## Benchmarks

Command:

```sh
go test -bench=. -benchmem ./internal/ratelimit
```

Sandbox command used for this run:

```sh
GOCACHE=$PWD/.gocache go test -bench=. -benchmem ./internal/ratelimit
```

Environment: Apple M1 Max, Darwin arm64, Go 1.25.10.

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| single-key token bucket | 18.87 | 0 | 0 |
| single-key fixed window | 29.74 | 0 | 0 |
| single-key sliding window | 55.04 | 125 | 0 |
| single-key leaky bucket | 28.94 | 0 | 0 |
| high-cardinality token bucket | 99.67 | 23 | 1 |
| high-cardinality fixed window | 103.9 | 23 | 1 |
| high-cardinality sliding window | 127.7 | 23 | 1 |
| high-cardinality leaky bucket | 125.8 | 23 | 1 |

## Production Notes

- Local limiters are process-local. Horizontal API scaling needs Redis or another shared counter store.
- Sliding window log can grow per hot key. In a shared store, use short TTLs and bounded lists.
- Token bucket is the best default for latency and allocation profile in the current local runtime.
- Leaky bucket is a better mental model for provider-facing worker pacing than tenant entitlement.
- Rate limiting decisions should be observable by endpoint, tenant, and limiter kind before production traffic.
