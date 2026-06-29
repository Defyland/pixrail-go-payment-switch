# Railway Deployment

PixRail includes `railway.json` for a single-service Railway deployment that
keeps the payment-switch API runnable as a public demo while documenting the
limits of the in-memory path.

## Runtime shape

- builder: `Dockerfile`
- process: `/pixrail-api`
- health check: `/readyz`
- default store: in-memory sandbox
- seeded demo keys: tenant, worker, risk, and provider roles

The Railway path proves the HTTP contract, role-scoped auth, idempotent
transfer creation, manual SPI submission, and observability endpoints from one
API process. It does not claim durable worker or broker behavior.

## Required variables

No variable is required for the minimal demo path. Railway injects `PORT`, and
PixRail now binds to that value automatically when `PIXRAIL_HTTP_ADDR` is not
set.

For a less fragile public demo, set:

```bash
PIXRAIL_API_KEYS=tenant_demo:<tenant-key>:tenant,tenant_demo:<worker-key>:worker,tenant_demo:<risk-key>:risk,tenant_demo:<provider-key>:provider
PIXRAIL_PROVIDER_CALLBACK_SECRET=<provider-callback-secret>
PIXRAIL_TRACING_EXPORTER=none
```

Optional production-like variables:

```bash
PIXRAIL_STORE_DRIVER=postgres
PIXRAIL_DATABASE_URL=<managed-postgres-url>
PIXRAIL_ENV=production
PIXRAIL_POSTGRES_MAX_CONNS=10
```

## Five-minute verification

After Railway deploys:

```bash
curl -fsS "$RAILWAY_PUBLIC_DOMAIN/healthz"
curl -fsS "$RAILWAY_PUBLIC_DOMAIN/readyz"
curl -fsS "$RAILWAY_PUBLIC_DOMAIN/metrics" | head
curl -fsS -X POST "$RAILWAY_PUBLIC_DOMAIN/v1/pix/transfers" \
  -H "Authorization: Bearer $PIXRAIL_TENANT_KEY" \
  -H "Idempotency-Key: railway-demo-1" \
  -H "Content-Type: application/json" \
  -d '{"account_id":"acct_123","amount_cents":12345,"currency":"BRL","receiver_key":"receiver@example.com","receiver_key_type":"EMAIL"}'
```

Expected create result: HTTP `202` with `accepted` status and no SPI
identifiers yet.

## Limits

- The default Railway path is single-process and in-memory.
- Durable PostgreSQL state, broker-backed outbox delivery, and independent
  worker processes still belong to the production-like topology.
- Public demo credentials should be replaced in Railway variables instead of
  relying on the repository defaults.
