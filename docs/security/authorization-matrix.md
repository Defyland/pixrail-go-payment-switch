# Authorization Matrix

| Endpoint | Required role | Tenant scope | Notes |
| --- | --- | --- | --- |
| `GET /healthz` | none | none | Process liveness only. |
| `GET /readyz` | none | none | Dependency readiness only. |
| `GET /metrics` | none in local runtime | none | Put behind network policy or auth proxy in production. |
| `POST /v1/pix/transfers` | `tenant` | API key tenant | Creates transfer only for the authenticated tenant. |
| `GET /v1/pix/transfers/{id}` | `tenant` | API key tenant | Cross-tenant reads return `404`. |
| `POST /v1/pix/transfers/{id}/spi-submissions` | `worker` | API key tenant | Operational worker action; tenant keys cannot trigger SPI side effects. |
| `POST /v1/pix/transfers/{id}/reviews` | `risk` | API key tenant | Manual risk decision path; tenant and worker keys are forbidden. |
| `POST /v1/pix/transfers/{id}/spi-callbacks` | `provider` plus HMAC callback signature | API key tenant | Requires `X-PixRail-Timestamp` and `X-PixRail-Signature` over `timestamp.body`. |
| `GET /v1/outbox` | `tenant` | API key tenant | Local inspection endpoint filtered by tenant; production should move this behind admin tooling. |

API keys are configured with `PIXRAIL_API_KEYS=tenant_id:secret[:role|role]`.

Examples:

```text
tenant_demo:dev-secret:tenant
tenant_demo:worker-secret:worker
tenant_demo:risk-secret:risk
tenant_demo:provider-secret:provider
tenant_demo:ops-secret:worker|provider
```

The legacy `tenant_id:secret` syntax is still accepted for local compatibility and grants only the `tenant` role. Production startup fails without configured keys.
