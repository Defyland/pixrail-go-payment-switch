# ADR 0007: Publish the Repository Under the MIT License

## Status

Accepted.

## Context

PixRail is a public payment-switch case study with Railway demo guidance,
idempotency behavior, role boundaries, and operational runbooks. Without an
explicit license, reviewers can inspect the repo but still lack a clear reuse
contract for internal learning or prototyping.

## Options considered

1. Keep the default copyright boundary
2. Publish under the MIT License
3. Wait until more production adapters are implemented

## Decision

Publish the repository under the MIT License now and document that clearly in
the README.

## Consequences

Positive:

- Teams can study and adapt the payment-switch patterns with a standard
  permissive license.
- The public demo and the public legal surface now tell the same story.

Negative:

- A downstream fork can omit the runbook and deployment constraints.
- License clarity does not replace the need to review third-party terms.
