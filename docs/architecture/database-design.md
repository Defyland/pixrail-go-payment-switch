# Database Design

The current implementation uses an in-memory store for local execution and tests. The production design maps directly to PostgreSQL.

## Tables

```sql
create table pix_transfers (
  id text primary key,
  tenant_id text not null,
  account_id text not null,
  idempotency_key text not null,
  correlation_id text not null,
  end_to_end_id text unique,
  amount_cents bigint not null check (amount_cents > 0),
  currency text not null check (currency = 'BRL'),
  receiver_key text not null,
  receiver_key_type text not null,
  receiver_name text not null,
  receiver_bank text not null,
  receiver_risk integer not null,
  fraud_score integer not null,
  fraud_rules jsonb not null default '[]',
  status text not null,
  decision_reason text not null,
  spi_message_id text unique,
  settlement_code text not null default '',
  created_at timestamptz not null,
  updated_at timestamptz not null,
  unique (tenant_id, idempotency_key)
);

create index pix_transfers_tenant_account_created_idx
  on pix_transfers (tenant_id, account_id, created_at desc);

create table payment_outbox (
  sequence bigserial primary key,
  event_id text not null unique,
  event_type text not null,
  schema_version text not null,
  tenant_id text not null,
  account_id text not null,
  pix_transfer_id text not null references pix_transfers(id),
  correlation_id text not null,
  payload jsonb not null,
  available_at timestamptz not null,
  published_at timestamptz,
  attempts integer not null default 0,
  last_error text not null default ''
);

create table audit_records (
  id bigserial primary key,
  tenant_id text not null,
  account_id text not null,
  pix_transfer_id text not null references pix_transfers(id),
  action text not null,
  correlation_id text not null,
  metadata jsonb not null,
  created_at timestamptz not null
);

create table processed_spi_callbacks (
  tenant_id text not null,
  spi_message_id text not null,
  callback_hash text not null,
  processed_at timestamptz not null,
  primary key (tenant_id, spi_message_id, callback_hash)
);
```

## Transaction boundaries

- Create transfer: insert transfer, audit record, and all outbox events in one transaction.
- Settlement callback: lock transfer by tenant and ID, validate SPI message ID, guard terminal states, update transfer, insert audit record, and insert settlement event.
- Outbox relay: publish pending events, then mark `published_at` after broker acknowledgement.

## Isolation and rollback

Default `READ COMMITTED` is acceptable with unique constraints and row-level locks on settlement updates. Failed DICT or fraud calls happen before the transaction, so no partial transfer row is written. Failed outbox insert rolls back the transfer decision.
