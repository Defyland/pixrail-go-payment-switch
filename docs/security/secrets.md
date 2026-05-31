# Secrets Management

PixRail uses environment variables for runtime secrets.

| Secret | Purpose | Required in production |
| --- | --- | --- |
| `PIXRAIL_API_KEYS` | maps `tenant_id:secret[:role|role]` API keys | yes |
| `PIXRAIL_PROVIDER_CALLBACK_SECRET` | signs and verifies provider callback bodies | yes |
| `PIXRAIL_PROVIDER_SIGNATURE_TOLERANCE` | accepted timestamp skew for provider callback signatures | no |
| `PIXRAIL_DATABASE_URL` | PostgreSQL DSN for durable state | yes for durable deployments |
| provider credentials | future DICT/SPI/broker adapters | yes when adapters are enabled |

## Rules

- Do not commit real secrets.
- Do not log API keys or DSNs.
- Disable the development `dev-secret`, `worker-secret`, `risk-secret`, and `provider-secret` fallbacks in production.
- Disable the development `dev-provider-callback-secret` fallback in production.
- Prefer separate keys per role instead of multi-role keys; use `worker|provider` style only for tightly controlled operational tooling.
- Rotate API keys through deployment configuration.
- Rotate the provider callback secret separately from API keys.
- Prefer short-lived provider credentials where external providers allow it.
