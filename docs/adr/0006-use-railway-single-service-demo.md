# ADR 0006: Use Railway for a Single-service PixRail Demo

## Status

Accepted.

## Context

PixRail is a runnable HTTP service and already exposes `/healthz`, `/readyz`,
and a Docker build path. The repository lacked a simple hosted demo surface,
which made the public-service story weaker than the local Compose story.

## Options considered

1. Do not add Railway support.
2. Add Railway for the API process only, using the in-memory or managed
   PostgreSQL path.
3. Require PostgreSQL plus the worker before any public demo deployment.

## Decision

Add Railway config as code for a single API process. The API now falls back to
Railway's injected `PORT` when `PIXRAIL_HTTP_ADDR` is unset. Keep the demo path
honest by documenting it as a hosted contract surface, not as proof of durable
worker or broker topology.

## Consequences

Positive:

- the repository gains a truthful public HTTP demo path;
- reviewers can exercise auth, idempotency, and transfer acceptance quickly;
- Railway can deploy without a dashboard-only bind-address override.

Negative:

- the default Railway path is still in-memory unless managed PostgreSQL is
  configured;
- the demo does not prove outbox relay or multi-process worker behavior;
- public demo keys still need to be replaced through Railway variables.

## Verification evidence

- `railway.json`
- `docs/deployment/railway.md`
- `internal/config/config.go`
- `internal/config/config_test.go`
