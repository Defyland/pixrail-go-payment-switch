# Serialization Benchmark

PixRail keeps HTTP and PostgreSQL JSON because they are operationally inspectable, easy to debug, and native to API/OpenAPI and `jsonb`. Binary codecs are still useful at internal boundaries: broker payloads, replay logs, and Redis-like caches where size and CPU matter.

## Scope

Benchmarked payload: a fixed PixRail payment event schema covering:

- `pix_transfer_requested`
- `pix_transfer_approved`
- `pix_transfer_blocked`
- Redis-like participant profile cache state derived from DICT resolution.

Code lives in `internal/codec`. The benchmark uses self-contained codecs so the repo stays runnable without `protoc` or external codec generators:

| Codec | Local implementation | Intended use |
| --- | --- | --- |
| JSON | Go `encoding/json` object | HTTP, docs, `jsonb`, incident inspection |
| Protobuf wire | Fixed schema over protobuf wire types | Internal broker/event payloads where schema is pinned |
| MsgPack | Fixed array schema | Redis-like cache values when compact binary state is useful |
| CBOR | Fixed array schema | Redis-like cache values when deterministic binary encoding is useful |

For a production provider/broker integration, a generated Protobuf package can replace the local wire codec without changing the benchmark contract.
For Redis, `ParticipantProfileMsgPackCodec` and `ParticipantProfileCBORCodec` encode concrete cache state and reject invalid TTL/risk values before writing binary bytes.

## Command

```sh
go test -bench=. -benchmem ./internal/codec
```

On this sandboxed run, `GOCACHE` was pointed into the repository because the global Go cache was not writable:

```sh
GOCACHE=$PWD/.gocache go test -bench=. -benchmem ./internal/codec
```

## Payment Event Results

Environment: Apple M1 Max, Darwin arm64, Go 1.25.10.

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| marshal JSON requested | 492.6 | 544 | 2 |
| marshal JSON approved | 642.8 | 704 | 2 |
| marshal JSON blocked | 614.9 | 672 | 2 |
| marshal Protobuf requested | 73.87 | 256 | 1 |
| marshal Protobuf approved | 79.65 | 256 | 1 |
| marshal Protobuf blocked | 83.21 | 256 | 1 |
| marshal MsgPack requested | 215.5 | 496 | 6 |
| marshal MsgPack approved | 183.8 | 496 | 6 |
| marshal MsgPack blocked | 179.3 | 496 | 6 |
| marshal CBOR requested | 177.1 | 488 | 5 |
| marshal CBOR approved | 195.5 | 488 | 5 |
| marshal CBOR blocked | 220.7 | 488 | 5 |
| roundtrip JSON approved | 3427 | 656 | 16 |
| roundtrip Protobuf approved | 278.5 | 216 | 11 |
| roundtrip MsgPack approved | 287.0 | 216 | 11 |
| roundtrip CBOR approved | 237.8 | 216 | 11 |

## Participant Profile Cache Results

Payload: receiver ID, ISPB, display name, risk signal, resolved timestamp, and expiry timestamp. This is the shape PixRail would store behind a Redis key such as `dict:{tenant}:{key_hash}`.

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| marshal JSON | 280.3 | 272 | 2 |
| marshal MsgPack profile cache | 106.0 | 184 | 5 |
| marshal CBOR profile cache | 108.8 | 176 | 4 |
| roundtrip MsgPack profile cache | 68.54 | 56 | 3 |
| roundtrip CBOR profile cache | 72.67 | 56 | 3 |

## Decision

- Keep JSON for public API, OpenAPI examples, audit-friendly `jsonb`, and local debugging.
- Prefer Protobuf wire payloads for high-volume internal payment events once a real broker adapter is added.
- Prefer MsgPack for compact Redis-like state such as routing-state snapshots and participant profile cache values when ecosystem support is already present.
- Prefer CBOR for Redis-like state when deterministic, schema-light binary payloads are more important than library ubiquity.
- Prefer the current in-memory token bucket structs for local rate limiting; encode rate-limit state only when moving it to Redis or another shared cache.
- Do not use binary codecs for audit records; audit evidence must remain directly inspectable.

## Gaps

- This benchmark does not claim compatibility with generated `.proto` APIs. It measures the payload shape and wire encoding cost that PixRail would use at an internal event boundary.
- Compression is intentionally excluded because payment events are small; compression is more likely to add p99 latency than reduce cost at this size.
