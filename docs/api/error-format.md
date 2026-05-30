# API Error Format

PixRail uses one error envelope for validation, authorization, rate-limit, conflict, not-found, dependency, and unexpected failures.

```json
{
  "error": {
    "code": "validation_failed",
    "message": "validation failed: amount_cents must be greater than zero",
    "details": null
  }
}
```

## Status mapping

| HTTP status | Code | Scenario |
| --- | --- | --- |
| 400 | `validation_failed` | invalid JSON, missing idempotency key, invalid amount, invalid key type |
| 401 | `unauthorized` | missing or invalid API key |
| 404 | `not_found` | transfer does not exist or belongs to another tenant |
| 409 | `conflict` | SPI message mismatch or duplicate conflicting write |
| 429 | `rate_limited` | tenant/account or DICT bucket exhausted |
| 502 | `dependency_failed` | DICT timeout or missing receiver key simulation |
| 500 | `internal_error` | unexpected server error |

`X-Request-ID` and `X-Correlation-ID` are returned on every response and must be copied into incident notes.
