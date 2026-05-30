# Authorization Matrix

| Endpoint | Authentication | Tenant scope | Notes |
| --- | --- | --- | --- |
| `GET /healthz` | none | none | process liveness only |
| `GET /readyz` | none | none | dependency readiness only |
| `GET /metrics` | none in local MVP | none | put behind network policy in production |
| `POST /v1/pix/transfers` | API key | API key tenant | creates transfer only for authenticated tenant |
| `GET /v1/pix/transfers/{id}` | API key | API key tenant | cross-tenant reads return `404` |
| `POST /v1/pix/transfers/{id}/spi-callbacks` | API key | API key tenant | local simulation; production should use signed provider callback |
| `GET /v1/outbox` | API key | API key tenant | local operations endpoint; production should be admin-only |

API keys are configured with `PIXRAIL_API_KEYS=tenant_id:secret`. Production startup fails without configured keys.
