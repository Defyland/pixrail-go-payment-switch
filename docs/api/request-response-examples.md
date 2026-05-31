# Request and Response Examples

## Create accepted transfer

```sh
curl -s -X POST http://localhost:8080/v1/pix/transfers \
  -H 'Authorization: Bearer dev-secret' \
  -H 'Idempotency-Key: example-1' \
  -H 'X-Correlation-ID: corr-example-1' \
  -H 'Content-Type: application/json' \
  -d '{"account_id":"acct_123","amount_cents":12345,"currency":"BRL","receiver_key":"receiver@example.com","receiver_key_type":"EMAIL"}'
```

```json
{
  "data": {
    "id": "pxt_...",
    "tenant_id": "tenant_demo",
    "account_id": "acct_123",
    "status": "accepted",
    "amount_cents": 12345,
    "currency": "BRL",
    "receiver_key_type": "EMAIL",
    "receiver_name": "Recebedor PixRail",
    "receiver_bank": "10000000",
    "fraud_score": 12,
    "fraud_rules": [],
    "decision_reason": "risk within payment-rail policy",
    "spi_message_id": "",
    "end_to_end_id": "",
    "settlement_code": "",
    "created_at": "2026-05-30T10:00:00Z",
    "updated_at": "2026-05-30T10:00:00Z"
  },
  "meta": {
    "idempotent_replay": false
  }
}
```

Create is deliberately pre-SPI: the transfer, idempotency fingerprint, audit record, and outbox events are durable before any SPI-style side effect.

## Submit accepted transfer to SPI

```sh
curl -s -X POST http://localhost:8080/v1/pix/transfers/pxt_123/spi-submissions \
  -H 'Authorization: Bearer dev-secret'
```

```json
{
  "data": {
    "id": "pxt_123",
    "status": "approved",
    "spi_message_id": "spi_...",
    "end_to_end_id": "E..."
  },
  "meta": {
    "idempotent_replay": false
  }
}
```

## Idempotent replay

Repeat the same request with the same `Idempotency-Key`. PixRail returns `200` and the same transfer without appending new outbox events.

Reusing the same `Idempotency-Key` with a different payload returns `409 conflict`; the stored request fingerprint is part of the transfer consistency boundary.

## Blocked transfer

```json
{
  "account_id": "acct_123",
  "amount_cents": 12345,
  "currency": "BRL",
  "receiver_key": "mule@example.com",
  "receiver_key_type": "EMAIL"
}
```

High-risk receiver keys return `status: blocked`; no SPI message is created.

## Manual review decision

```sh
curl -s -X POST http://localhost:8080/v1/pix/transfers/pxt_123/reviews \
  -H 'Authorization: Bearer dev-secret' \
  -H 'Content-Type: application/json' \
  -d '{"decision":"approve","reason":"analyst approved after review"}'
```

Approving a review moves the transfer back to `accepted` and emits a new `spi_submission_requested` event. Blocking a review moves it to `blocked`.

## Settlement callback

```sh
curl -s -X POST http://localhost:8080/v1/pix/transfers/pxt_123/spi-callbacks \
  -H 'Authorization: Bearer dev-secret' \
  -H 'Content-Type: application/json' \
  -d '{"spi_message_id":"spi_123","status":"accepted","code":"ACSC"}'
```

Accepted callbacks move an approved transfer to `settled`. Repeated callbacks with the same callback hash replay the terminal state. A terminal callback with a different SPI message ID or conflicting callback payload returns `409 conflict`.

## Authorization failure

```sh
curl -s -X POST http://localhost:8080/v1/pix/transfers \
  -H 'Content-Type: application/json' \
  -d '{}'
```

```json
{
  "error": {
    "code": "unauthorized",
    "message": "valid API key is required",
    "details": null
  }
}
```

## Validation failure

```sh
curl -s -X POST http://localhost:8080/v1/pix/transfers \
  -H 'Authorization: Bearer dev-secret' \
  -H 'Idempotency-Key: invalid-1' \
  -H 'Content-Type: application/json' \
  -d '{"amount_cents":0,"currency":"USD","receiver_key_type":"EMAIL"}'
```

```json
{
  "error": {
    "code": "validation_failed",
    "message": "validation failed: account_id is required; amount_cents must be greater than zero; currency must be BRL; receiver_key is required",
    "details": null
  }
}
```
