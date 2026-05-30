# Use Cases

## Create a Pix transfer

Tenant submits a transfer intent with an idempotency key. PixRail validates the payload, resolves the receiver key, scores fraud risk, applies rate limits, creates a SPI-style message when approved, and writes outbox events.

## Replay a duplicate request

Tenant retries with the same idempotency key. PixRail returns the original transfer and does not append duplicate events.

## Block a high-risk receiver

DICT risk or fraud rules push the score above the blocking threshold. PixRail records the decision and publishes a block event without creating a SPI message.

## Record a SPI callback

Payment-network callback marks an approved transfer as settled or rejected. Duplicate callbacks for terminal transfers replay the terminal state.

## Operate event delivery

Outbox relay fetches pending events, publishes them to a downstream broker, marks acknowledged events as published, and schedules failed publishes for retry with evidence.
