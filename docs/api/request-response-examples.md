# Request and Response Examples

## Create approved transfer

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
    "status": "approved",
    "amount_cents": 12345,
    "currency": "BRL",
    "receiver_key_type": "EMAIL",
    "receiver_name": "Recebedor PixRail",
    "receiver_bank": "10000000",
    "fraud_score": 12,
    "fraud_rules": [],
    "decision_reason": "risk within payment-rail policy",
    "spi_message_id": "spi_...",
    "end_to_end_id": "E...",
    "settlement_code": "",
    "created_at": "2026-05-30T10:00:00Z",
    "updated_at": "2026-05-30T10:00:00Z"
  },
  "meta": {
    "idempotent_replay": false
  }
}
```

## Idempotent replay

Repeat the same request with the same `Idempotency-Key`. PixRail returns `200` and the same transfer without appending new outbox events.

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

## Settlement callback

```sh
curl -s -X POST http://localhost:8080/v1/pix/transfers/pxt_123/spi-callbacks \
  -H 'Authorization: Bearer dev-secret' \
  -H 'Content-Type: application/json' \
  -d '{"spi_message_id":"spi_123","status":"accepted","code":"ACSC"}'
```

Accepted callbacks move an approved transfer to `settled`. Repeated callbacks for a terminal transfer replay the terminal state.
