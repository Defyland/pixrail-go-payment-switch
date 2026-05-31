# Dependency Rule

PixRail dependencies point inward. Primary adapters call use cases. Use cases call ports and domain. Secondary adapters implement ports. Domain does not import application or infrastructure.

## Allowed Direction

```text
cmd/*, internal/app
  -> internal/api, internal/switcher, internal/postgres, internal/store, internal/dict,
     internal/fraud, internal/spi, internal/ratelimit, internal/observability

internal/api
  -> internal/switcher, internal/rail, internal/events, internal/config, internal/observability

internal/switcher
  -> internal/rail, internal/events

internal/postgres, internal/store, internal/dict, internal/fraud, internal/spi
  -> internal/rail and, where needed, internal/events

internal/rail
  -> standard library only
```

## Enforced Checks

`internal/spec/architecture_spec_test.go` parses production Go imports and fails if:

- `internal/rail` or `internal/events` import any internal PixRail package.
- `internal/switcher` imports anything beyond `internal/rail` and `internal/events`.
- `internal/api` imports persistence/provider/cache adapters.
- secondary adapters import primary adapters, composition root, or use cases.

This is intentionally stricter than a generic Go project. The point is to keep PixRail readable as a payment-switch reference, not as an MVC codebase with renamed folders.

## Practical Rule For New Code

- Adding a new HTTP endpoint starts in `internal/api`, but the behavior belongs in a use case.
- Adding a new payment state transition starts in `internal/rail`.
- Adding a provider starts as an adapter that satisfies a port declared by the use case.
- Adding storage changes belongs in a secondary adapter plus migration, not in `internal/api`.
- Adding an event starts with a versioned schema in `docs/events` and event emission in the use case.

## Acceptable Exceptions

`internal/app` and `cmd/*` are composition/process boundaries and may know concrete adapters. Tests may import concrete adapters to provide integration evidence, but production dependency rules remain enforced by the spec test.
