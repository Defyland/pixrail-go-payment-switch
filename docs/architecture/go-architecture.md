# Go Architecture

PixRail uses Go packages as architecture boundaries. The design is a modular monolith, not MVC: packages are grouped by responsibility and dependency direction, not by generic `controllers`, `models`, and `repositories`.

## Shape

```text
cmd/
  pixrail-api        process adapter
  pixrail-worker     process adapter
  pixrail-migrate    migration adapter

internal/api         HTTP primary adapter
internal/app         composition root
internal/switcher    payment switch use cases and ports
internal/rail        domain model and state machine
internal/events      versioned event envelope
internal/postgres    PostgreSQL secondary adapter
internal/store       in-memory secondary adapter
internal/dict        local participant resolver adapter
internal/fraud       local fraud scoring adapter
internal/spi         local SPI adapter
internal/ratelimit   rate-limit algorithms/adapters
internal/codec       binary payload/cache codecs
internal/messaging   outbox relay use case and publisher port
internal/observability platform metrics/tracing adapter
```

## Go Decisions

| Decision | Reason |
| --- | --- |
| Standard `net/http` at the edge | Keeps framework behavior small and explicit. |
| Interfaces declared by consumers | `switcher` declares ports based on use case needs; adapters satisfy them implicitly. |
| Composition in `internal/app` | Main packages wire concrete adapters; use cases do not know which adapter was selected. |
| Domain methods on `rail.Transfer` | Payment state transitions are testable without HTTP or persistence. |
| Thin handlers | HTTP DTOs are decoded and mapped once; they do not cross into storage or domain as transport structs. |
| Versioned events | Event payloads are contracts, not incidental log messages. |

## Why This Is Not MVC

MVC would normally put HTTP controllers over a service/repository stack with database-shaped models. PixRail deliberately avoids that:

- `internal/api` does not import PostgreSQL, in-memory store, DICT, fraud, SPI, rate-limit, or codec adapters.
- `internal/switcher` does not import primary adapters or secondary adapter packages.
- `internal/rail` imports no PixRail internal packages.
- PostgreSQL code maps rows to domain state and calls domain transition methods instead of owning payment status rules.
- Use case tests inject fake ports and can run without a router, database, cache, or broker.

## Framework At The Edge

The only framework-like surface is the standard HTTP router. OpenTelemetry, Prometheus, PostgreSQL, pprof, and Docker are edge/platform concerns. They are useful production controls, but they do not define the domain model or use case boundaries.
