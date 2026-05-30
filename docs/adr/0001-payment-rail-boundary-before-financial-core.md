# ADR 0001: Keep PixRail as a Payment Rail, Not the Financial Core

## Status

Accepted.

## Context

PixRail processes hot-path Pix traffic. SettleFlow owns ledger, balances, settlement, payouts, refunds, and reconciliation.

## Decision

PixRail decides, rate-limits, routes, simulates SPI messaging, and emits payment-rail events. It does not own double-entry ledger records or customer balances.

## Consequences

- PixRail can optimize for low latency and backpressure without carrying ledger complexity.
- SettleFlow remains the source of truth for money.
- Contracts between the systems are event-based and versioned.
- Financial reconciliation happens outside PixRail.
