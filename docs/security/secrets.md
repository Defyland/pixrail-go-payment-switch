# Secrets Management

PixRail uses environment variables for runtime secrets.

| Secret | Purpose | Required in production |
| --- | --- | --- |
| `PIXRAIL_API_KEYS` | maps `tenant_id:secret` API keys | yes |
| `PIXRAIL_DATABASE_URL` | PostgreSQL DSN for durable state | yes for durable deployments |
| provider credentials | future DICT/SPI/broker adapters | yes when adapters are enabled |

## Rules

- Do not commit real secrets.
- Do not log API keys or DSNs.
- Disable the development `dev-secret` fallback in production.
- Rotate API keys through deployment configuration.
- Prefer short-lived provider credentials where external providers allow it.
