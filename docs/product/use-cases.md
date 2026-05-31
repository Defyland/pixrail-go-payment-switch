# Use Cases

## Create a Pix transfer

Tenant submits a transfer intent with an idempotency key. PixRail validates the payload, resolves the receiver key, scores fraud risk, applies rate limits, stores a request fingerprint, records an `accepted`, `review`, or `blocked` transfer, and writes outbox events before any SPI-style side effect.

## Replay a duplicate request

Tenant retries with the same idempotency key. PixRail returns the original transfer and does not append duplicate events. Reusing the same key with a different request payload returns a conflict.

## Submit to SPI

An accepted transfer is submitted to the SPI simulator only after the transfer is durable. PixRail records SPI identifiers and approval events; replaying the same SPI submission returns the already approved transfer.

## Resolve manual review

An analyst approves or blocks a transfer in `review`. Approval returns it to `accepted` and requests SPI submission; blocking makes it terminal without creating a SPI message.

## Block a high-risk receiver

DICT risk or fraud rules push the score above the blocking threshold. PixRail records the decision and publishes a block event without creating a SPI message.

## Record a SPI callback

Payment-network callback marks an approved transfer as settled or rejected. Duplicate callbacks replay only when the SPI message ID and callback hash match the processed callback.

## Operate event delivery

Outbox relay fetches pending events, publishes them to a downstream broker, marks acknowledged events as published, and schedules failed publishes for retry with evidence.
