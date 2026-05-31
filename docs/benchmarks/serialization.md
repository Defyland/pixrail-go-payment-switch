# Serialization Benchmark

PixRail keeps HTTP and PostgreSQL JSON because they are operationally inspectable, easy to debug, and native to API/OpenAPI and `jsonb`. Binary codecs are still useful at internal boundaries: broker payloads, replay logs, and Redis-like caches where size and CPU matter.

## Scope

Benchmarked payload: a fixed PixRail payment event schema covering:

- `pix_transfer_requested`
- `pix_transfer_approved`
- `pix_transfer_blocked`

Code lives in `internal/codec`. The benchmark uses self-contained codecs so the repo stays runnable without `protoc` or external codec generators:

| Codec | Local implementation | Intended use |
| --- | --- | --- |
| JSON | Go `encoding/json` object | HTTP, docs, `jsonb`, incident inspection |
| Protobuf wire | Fixed schema over protobuf wire types | Internal broker/event payloads where schema is pinned |
| MsgPack | Fixed array schema | Redis-like cache values when compact binary state is useful |
| CBOR | Fixed array schema | Redis-like cache values when deterministic binary encoding is useful |

For a production provider/broker integration, a generated Protobuf package can replace the local wire codec without changing the benchmark contract.

## Command

```sh
go test -bench=. -benchmem ./internal/codec
```

On this sandboxed run, `GOCACHE` was pointed into the repository because the global Go cache was not writable:

```sh
GOCACHE=$PWD/.gocache go test -bench=. -benchmem ./internal/codec
```

## Results

Environment: Apple M1 Max, Darwin arm64, Go 1.25.10.

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| marshal JSON requested | 554.1 | 544 | 2 |
| marshal JSON approved | 1915 | 704 | 2 |
| marshal JSON blocked | 1131 | 672 | 2 |
| marshal Protobuf requested | 77.00 | 256 | 1 |
| marshal Protobuf approved | 99.91 | 256 | 1 |
| marshal Protobuf blocked | 100.5 | 256 | 1 |
| marshal MsgPack requested | 170.4 | 496 | 6 |
| marshal MsgPack approved | 164.0 | 496 | 6 |
| marshal MsgPack blocked | 168.5 | 496 | 6 |
| marshal CBOR requested | 170.8 | 488 | 5 |
| marshal CBOR approved | 168.2 | 488 | 5 |
| marshal CBOR blocked | 189.5 | 488 | 5 |
| roundtrip JSON approved | 3463 | 656 | 16 |
| roundtrip Protobuf approved | 254.1 | 216 | 11 |
| roundtrip MsgPack approved | 222.8 | 216 | 11 |
| roundtrip CBOR approved | 253.2 | 216 | 11 |

## Decision

- Keep JSON for public API, OpenAPI examples, audit-friendly `jsonb`, and local debugging.
- Prefer Protobuf wire payloads for high-volume internal payment events once a real broker adapter is added.
- Prefer MsgPack for compact Redis-like state such as routing-state snapshots and participant profile cache values.
- Prefer the current in-memory token bucket structs for local rate limiting; encode rate-limit state only when moving it to Redis or another shared cache.
- Do not use binary codecs for audit records; audit evidence must remain directly inspectable.

## Gaps

- This benchmark does not claim compatibility with generated `.proto` APIs. It measures the payload shape and wire encoding cost that PixRail would use at an internal event boundary.
- Compression is intentionally excluded because payment events are small; compression is more likely to add p99 latency than reduce cost at this size.
